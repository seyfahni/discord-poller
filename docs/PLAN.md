# Discord Poller — Implementation Plan

Status: planning only; implementation has not started. Remaining decisions are listed below.

## Scope

Build a Go command-line program that runs once, posts one native Discord poll using an incoming webhook, then exits. A server-side systemd timer invokes it weekly; manual invocation is also supported. The timer's execution weekday is independent of the poll's target weekday.

Configuration, CLI help, errors, logs, and operational documentation are English. Poll question and answer text are supplied verbatim by the administrator. No locale engine or translation support is required.

Out of scope: bots, extra message content, mentions, webhook username/avatar overrides, and additional placeholders at initial release.

## Configuration proposal

Use a strict YAML configuration file. Reject unknown keys to catch mistakes. Read the webhook URL from the named environment variable rather than embedding a secret in version control.

```yaml
webhook_url_env: DISCORD_WEBHOOK_URL

target:
  timezone: Europe/Zurich  # Example only; timezone still needs confirmation.
  weekday: Thursday
  time: "18:00"
  same_day_threshold: 3h

poll:
  question: "Who's in on {end_date}?"
  date_format: "02.01.2006"
  allow_multiselect: false
  answers:
    - text: "I'm in"
      emoji: "✅"
    - text: "Can't make it"
      emoji: "❌"
    - text: "Maybe"
```

- Target weekday accepts full English weekday names.
- Target time uses local 24-hour `HH:MM` syntax.
- Threshold is a Go duration string, defaults to `3h`, and must be at least `1h`. Proposed upper bound: less than one week; larger thresholds make same-day selection impossible or confusing.
- Timezone is an explicit IANA timezone; do not silently inherit the server timezone.
- Date format uses Go reference-time layouts, e.g. `02.01.2006`, `2006-01-02`, or `Monday, 02 January 2006`. Go's built-in weekday/month names are English; localized literal words and numeric layouts can be supplied, but translated dynamic names are out of scope.
- Answer text is required. Emoji is optional. Proposed support: Unicode emoji and custom Discord emoji notation (`<:name:id>` and `<a:name:id>`), mapped to the appropriate API fields. Do not equate one emoji with one Unicode code point: combined emoji sequences exist.
- `allow_multiselect` is configurable and defaults to false.

## Target selection

Capture the actual execution instant once and convert it into the configured timezone. Inject this clock in tests.

1. Find the target weekday on or after the local execution date.
2. Construct the configured wall-clock deadline on that calendar date.
3. If today is the target weekday, select today only when:
   `execution_time < deadline - same_day_threshold`.
4. Otherwise, select that weekday in the following calendar week.
5. On other weekdays, select the next occurrence of the configured weekday.

This interprets the user's explicit example as requiring *more than* the threshold of remaining time, not running inside the final threshold window. At the exact cutoff, choose the following week; this boundary remains subject to confirmation.

With Thursday, 18:00, and a 3h threshold:

| Execution (target timezone) | Selected deadline |
| --- | --- |
| Friday, October 2, 2026 | Thursday, October 8, 2026 at 18:00 |
| Monday, October 5, 2026 | Thursday, October 8, 2026 at 18:00 |
| Thursday, October 8, 2026 at 14:59:59 | Thursday, October 8, 2026 at 18:00 |
| Thursday, October 8, 2026 at 15:00:00 | Thursday, October 15, 2026 at 18:00 |
| Thursday, October 8, 2026 at or after 18:00 | Thursday, October 15, 2026 at 18:00 |

Use local calendar arithmetic, not adding a fixed 168 hours, to preserve the configured wall-clock time across daylight-saving changes. Compute elapsed durations between absolute instants after constructing the local deadline.

Proposed DST policy: reject ambiguous or nonexistent target wall-clock deadlines with a clear English error rather than silently choosing or normalizing an instant. The current 18:00 value is normally unaffected, but the configurable setting needs a documented policy.

## Poll duration and expiry window

Let `T` be the selected configured deadline and `S` the instant just before a send attempt. Compute the integer duration using:

`duration_hours = floor((T - S) / 1 hour)`.

The nominal expiry `S + duration_hours * 1 hour` then satisfies:

`T - 1 hour < nominal_expiry <= T`.

For an 18:00 target, nominal expiry is strictly after 17:00 and no later than 18:00. An exact whole-hour difference can end exactly at 18:00; do not subtract an unconditional extra hour.

Important limitation: Discord starts the duration when it creates the poll, not at the local calculation instant. Transport/server delay can move actual expiry slightly later, especially when nominal expiry equals the deadline. A webhook-only program cannot guarantee this strict window under arbitrary delay. A fixed safety margin alone also cannot guarantee both window boundaries for every execution phase.

Proposed operational behavior:
- Recalculate duration immediately before each permitted send attempt, keeping the chosen target date fixed for that invocation.
- Abort if the remaining duration falls below one hour or the supported range; never silently change to a different target during retries.
- Use `wait=true` to obtain the created message and inspect returned `poll.expiry` when present.
- Log the actual returned expiry and warn/report a window violation. Do not repost to correct it, since that would create duplicates.
- Ask the user whether best-effort expiry with explicit reporting is acceptable.

`{end_date}` refers to the selected configured deadline in the configured timezone, not Discord's rounded actual expiry. Format it with `poll.date_format` before rendering the question.

## Template design

Initially support only `{end_date}` in the question. Use an isolated placeholder resolver so additional named placeholders can be introduced later without rewriting scheduling or payload code. Reject unsupported placeholders with a clear error. Answer texts remain literal initially. Define/document brace escaping during implementation.

## Discord integration and validation

Use the native webhook `poll` payload, without extra message content. API documentation currently specifies:
- Integer duration in hours, maximum 32 days (768 hours).
- Up to 10 answers.
- Question text maximum 300 characters; answer text maximum 55 characters.
- Optional answer emoji: Unicode emoji through `name`, custom emoji through `id`.
- Configurable `allow_multiselect`.

Project policy: require a nonempty rendered question, 1–10 nonempty answers, and duration of 1–768 hours. Confirm the single-answer case in integration testing; the documented upper limit does not establish a lower limit. Validate text using Unicode-aware counting rather than byte lengths, and verify Discord's counting semantics during implementation.

Perform configuration and rendered-payload validation before any network request. Validate webhook URL shape and HTTPS. Never log the webhook URL/token, including errors emitted by the HTTP client; sanitize error details.

Use an HTTP client with explicit timeouts and request cancellation. Respect Discord rate-limit responses with a bounded retry policy based on its current documentation. Do not blindly retry timeouts, connection failures after sending, or server errors: the original request might already have created a poll. Report ambiguous outcomes to the operator rather than risking an automatic duplicate.

## CLI and architecture

Proposed CLI: `discord-poller --config /etc/discord-poller/config.yaml`.

Add `--dry-run` to validate, select the target, and print payload plus timing calculations without sending or exposing credentials. Dry-run should not require the webhook secret.

Suggested components:
- `cmd/discord-poller`: CLI and composition.
- `internal/config`: strict configuration parsing and validation.
- `internal/schedule`: timezone/calendar selection and duration calculation.
- `internal/template`: extensible named-placeholder rendering.
- `internal/poll`: Discord request types and payload construction.
- `internal/discord`: webhook client, response parsing, rate limiting, secret-safe errors.

Prefer the Go standard library for time handling, HTTP, JSON, and logging. Select a maintained YAML library at implementation time. Avoid a large Discord SDK for one endpoint.

Log in English to stdout/stderr for collection by the service manager. Include selected deadline, requested duration, actual expiry/message ID when returned, and success/failure. Use nonzero exit status on failure.

## systemd deployment

Provide a oneshot service and editable weekly timer example. The service executes the binary once and does not contain an internal scheduler. The administrator chooses timer weekday/time independently of the configured target weekday/time.

Proposed deployment includes a dedicated unprivileged account, read-only configuration, separately stored webhook environment secret, service hardening compatible with outbound networking, and installation/verification instructions.

No automatic service restart on ambiguous delivery failures. Timer catch-up behavior (`Persistent=`) and duplicate protection remain open decisions. If catch-up is enabled, calculate from the actual execution time, not the originally missed timer timestamp.

## Tests and acceptance criteria

- Table-driven tests covering every target weekday, before/at/after cutoff, exact deadline, midnight, month/year boundaries, and fractional-hour thresholds.
- Friday/Monday executions resolve to the same upcoming deadline as illustrated.
- Thresholds below one hour fail validation.
- Property tests: nominal expiry is strictly later than deadline minus one hour and at most the deadline.
- Exact hour, fractional hour, and subsecond duration cases.
- Timezone conversion and calendar arithmetic across DST; explicit ambiguous/nonexistent local-time policy tests.
- Template replacement, unsupported placeholders, Unicode strings, and post-render length validation.
- Answers with no emoji, Unicode sequences, and custom emoji IDs; multiselect on/off.
- HTTP tests with a local test server: success, API errors, rate limits, timeout/ambiguous outcomes, and token redaction.
- Verify returned expiry handling and no corrective reposts.
- Dry-run makes no network call and needs no webhook credential.
- Document and manually test deployment with the systemd timer.

## Delivery phases

1. Save and confirm this plan and outstanding decisions.
2. Implement Go CLI, scheduling/config/template logic, payload construction, and unit tests.
3. Implement webhook client and HTTP tests.
4. Add example configuration, systemd units, build/install instructions, and CI.
5. Perform an opt-in live test with a user-provided webhook secret; never commit that secret.

Implementation should be submitted for review only after the user asks to start it.

## Open questions

1. Which IANA timezone should be used? `Europe/Zurich` above is an unconfirmed example.
2. Confirm strict cutoff: at exactly 15:00 with Thursday 18:00 and threshold 3h, choose the following Thursday?
3. Is best-effort actual expiry acceptable given Discord's server-side start time and network delay, with returned expiry checked and deviations reported?
4. Which weekday/time should the example weekly systemd timer use? Should missed runs be caught up after downtime?
5. Should repeated successful invocations for the same target deadline be suppressed using local state, or should each invocation create a poll? Local state cannot provide exactly-once delivery after an ambiguous network failure.
6. Are native Go date layouts acceptable? They permit custom numeric/literal formatting but not translated dynamic weekday/month names without additional support.

## Primary references

- Discord poll resource: https://docs.discord.com/developers/resources/poll
- Discord execute webhook: https://docs.discord.com/developers/resources/webhook#execute-webhook
- Go time formatting and timezones: https://pkg.go.dev/time

Recheck current API constraints and deployment directives during implementation.

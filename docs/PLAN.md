# Discord Poller — Implementation Plan

Status: planning complete; implementation ready to start.

## Scope

Build a Go command-line program that runs once, posts one native Discord poll using an incoming webhook, then exits. A server-side systemd timer invokes it weekly; manual invocation is also supported. The timer's execution weekday is independent of the poll's target weekday.

Configuration, CLI help, errors, logs, and operational documentation are English. Poll question and answer text are supplied verbatim by the administrator. No locale engine or translation support is required.

Out of scope: bots, extra message content, mentions, webhook username/avatar overrides, additional placeholders, and duplicate prevention via local state at initial release.

## Configuration proposal

Use a strict YAML configuration file. Reject unknown keys to catch mistakes. Read the webhook URL from the named environment variable rather than embedding a secret in version control.

```yaml
webhook_url_env: DISCORD_WEBHOOK_URL

target:
  timezone: Europe/Berlin
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

- **Timezone**: IANA timezone string (e.g. `Europe/Berlin`). No default; must be explicitly configured. Required setting.
- **Target weekday**: Full English weekday name. Required setting.
- **Target time**: Local 24-hour `HH:MM` syntax. Required setting.
- **Same-day threshold**: Go duration string (e.g. `3h`, `2h30m`). Minimum `1h`. Required setting.
- **Date format**: Go reference-time layout (e.g. `02.01.2006`, `2006-01-02`, `Monday, 02 January 2006`). Use numeric output only; English dynamic names (Monday, January) are acceptable. Required setting.
- **Question**: Template string supporting `{end_date}` placeholder. Required setting.
- **Answers**: At least one answer. Text is required; emoji is optional. Required setting (non-empty array).
- **Multiselect**: Boolean. Required setting.

Answer text is required. Emoji is optional. Proposed support: Unicode emoji and custom Discord emoji notation (`<:name:id>` and `<a:name:id>`), mapped to the appropriate API fields. Do not equate one emoji with one Unicode code point: combined emoji sequences exist.

## Target selection logic

Capture the actual execution instant once and convert it into the configured timezone. Inject this clock in tests.

1. Find the target weekday on or after the local execution date.
2. Construct the configured wall-clock deadline on that calendar date.
3. If today is the target weekday, select today only when:
   `execution_time < (deadline - same_day_threshold)`.
   That is, strictly before the cutoff time.
4. Otherwise, select that weekday in the following calendar week.
5. On other weekdays, select the next occurrence of the configured weekday.

With Thursday, 18:00, and a 3h threshold (cutoff at 15:00):

| Execution (target timezone) | Selected deadline |
| --- | --- |
| Friday, October 2, 2026 | Thursday, October 8, 2026 at 18:00 |
| Monday, October 5, 2026 | Thursday, October 8, 2026 at 18:00 |
| Thursday, October 8, 2026 at 14:59:59 | Thursday, October 8, 2026 at 18:00 |
| Thursday, October 8, 2026 at 15:00:00 | Thursday, October 15, 2026 at 18:00 |
| Thursday, October 8, 2026 at or after 18:00 | Thursday, October 15, 2026 at 18:00 |

Use local calendar arithmetic, not adding a fixed 168 hours, to preserve the configured wall-clock time across daylight-saving changes. Compute elapsed durations between absolute instants after constructing the local deadline.

**DST policy**: Reject ambiguous or nonexistent target wall-clock deadlines with a clear English error rather than silently choosing or normalizing an instant. The current 18:00 value is normally unaffected, but the configurable setting needs this documented policy.

## Poll duration and expiry window

Let `T` be the selected configured deadline and `S` the instant just before a send attempt. Compute the integer duration using:

`duration_hours = floor((T - S) / 1 hour)`.

The nominal expiry `S + duration_hours * 1 hour` then satisfies:

`T - 1 hour < nominal_expiry <= T`.

For an 18:00 target, nominal expiry is strictly after 17:00 and no later than 18:00. An exact whole-hour difference can end exactly at 18:00; do not subtract an unconditional extra hour.

**Important limitation**: Discord starts the duration when it creates the poll, not at the local calculation instant. Transport/server delay can move actual expiry slightly later, especially when nominal expiry equals the deadline.

**Operational behavior**:
- Recalculate duration immediately before each permitted send attempt, keeping the chosen target date fixed for that invocation.
- Abort if the remaining duration falls below one hour or exceeds the supported range (1–768 hours); never silently change to a different target during retries.
- Use `wait=true` to obtain the created message and inspect returned `poll.expiry` when present.
- Log the actual returned expiry and report any deviation from the intended window. Do not repost to correct it, since that would create duplicates.
- Best-effort expiry with explicit reporting is the acceptable approach.

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

### Service

Run the program as a oneshot service with an unprivileged account. The service executes the binary once and does not contain an internal scheduler.

### Timer

Run weekly on **Thursday at 18:10** (Berlin time, daylight-saving-aware). This gives approximately one week for the poll to exist if everything succeeds. Missed runs are **not** caught up (`Persistent=false`).

Rationale:
- Thursday 18:10 is 10 minutes after the default configured target deadline (18:00), giving a small operational buffer.
- The poll is then available for ~7 days until the next Thursday 18:00 deadline.
- If the timer is missed (e.g., system downtime), the next scheduled run (following Thursday) is used; stale deadlines are not retroactively executed.

Example systemd timer:
```ini
[Unit]
Description=Weekly Discord Poller Timer
Documentation=file:///etc/discord-poller/README.md

[Timer]
OnCalendar=Thu *-*-* 18:10:00
Persistent=false
AccuracySec=1min

[Install]
WantedBy=timers.target
```

Provide a oneshot service unit, example timer configuration, a dedicated unprivileged account, read-only configuration directory, separately stored webhook environment secret (e.g., in a systemd EnvironmentFile), service hardening compatible with outbound networking, and installation/verification instructions.

## Duplicate handling

**No local state tracking** at this release. Each invocation is independent and stateless:
- Manual invocation and scheduled timer on the same target deadline will create two separate polls.
- Repeated invocations do not suppress or detect duplicates.

This is acceptable because:
- Missed or concurrent executions are rare in typical deployments.
- Local state cannot guarantee exactly-once delivery after ambiguous network failures anyway.
- Stateless operation is simpler and avoids complex edge cases.

Future releases may add optional idempotency tokens or local state if required.

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

1. ✅ Planning complete with all decisions clarified.
2. Implement Go CLI, scheduling/config/template logic, payload construction, and unit tests.
3. Implement webhook client and HTTP tests.
4. Add example configuration, systemd units, build/install instructions, and CI.
5. Perform an opt-in live test with a user-provided webhook secret; never commit that secret.

Implementation should be submitted for review only after the user asks to start it.

## Primary references

- Discord poll resource: https://docs.discord.com/developers/resources/poll
- Discord execute webhook: https://docs.discord.com/developers/resources/webhook#execute-webhook
- Go time formatting and timezones: https://pkg.go.dev/time
- systemd timer: https://www.freedesktop.org/software/systemd/man/latest/systemd.timer.html

Recheck current API constraints and deployment directives during implementation.

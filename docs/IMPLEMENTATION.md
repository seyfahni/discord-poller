# Discord Poller — Detailed Implementation Plan

Status: planned implementation phases 1–4 complete.

Completed: configuration models, validation and strict YAML loaders;
scheduling deadline selection and duration calculations; template rendering in
question and answer text; Discord poll payloads and webhook HTTP client; CLI
composition and dry-run behavior.

For templates, `{{` and `}}` produce literal braces. Reject unmatched braces
and unsupported placeholders. Render `{end_date}` in both question and answer
text; keep Discord-specific rendered-length validation in the payload layer.
Next: opt-in live testing with a user-provided webhook credential.

### Webhook URL configuration adjustment

Store the required `webhook_url` directly in the user YAML instead of naming a
separate credential source. Keep URL-shape validation in the Discord layer and never
log the URL or token. Update config validation/loader tests, CLI tests for
missing, blank, and invalid URLs, the example config, README, and both project
plans. Verify the configured URL is passed to the Discord client while
credentials remain absent from logs and errors.

### Discord integration slice

Implement one file at a time, test-first:

1. `internal/discord/discord_test.go`: cover payload constraints and JSON shape;
   Unicode/custom emoji mapping; webhook URL validation; GET validation;
   `wait=true` poll creation and response expiry; 429 retries with numeric
   `Retry-After`; missing, invalid, over-budget, and exhausted rate limits;
   context cancellation; API rejection; server/network ambiguity without retry;
   and credential redaction.
2. `internal/discord/payload.go`: construct and validate the poll request
   before network activity. Count Unicode code points (the chosen assumption
   because Discord does not publish the counting unit), reject invalid UTF-8,
   enforce question/answer/duration limits, and map Discord emoji fields.
3. `internal/discord/response.go`: decode only the created message ID and
   optional poll expiry.
4. `internal/discord/client.go`: validate HTTPS webhook URL shape, perform
   authenticated-by-token GET validation, POST with `wait=true`, and retry only
   HTTP 429 responses within configured retry and time budgets. Require a valid
   numeric `Retry-After`; missing or invalid values fail without a guessed
   delay. Never retry connection failures or server errors for poll creation.
   Redact the webhook URL/token from errors and do not follow redirects.

Use injectable HTTP clients and context-aware rate-limit waiting for tests.
Return the created expiry to the caller; the CLI will log it and report window
deviations without reposting. Dry-run performs webhook GET validation but never
sends the poll-creation POST.

## Overview

This document breaks down the implementation into concrete phases, testing strategy, file structure, and configuration schemas. All decisions from `docs/PLAN.md` are incorporated, along with clarifications on logging, configuration splitting, and exit codes.

## Configuration architecture

### User configuration vs. System configuration

**User configuration** (`user-config.yaml`): Poll content, target timing, timezone.  
**System configuration** (`system-config.yaml`): Logging, HTTP timeouts, and retry behavior.

Both are required. Users manage user-config; operators maintain system-config.
The webhook URL is stored in user-config, which must be access-restricted and
must not be committed.

### User configuration schema

File: `user-config.yaml`

All fields are required. No defaults. Unknown keys are rejected.

```yaml
webhook_url: https://discord.com/api/webhooks/ID/TOKEN

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

**Validation rules**:
- `webhook_url`: nonempty string; the CLI validates HTTPS Discord webhook URL
  shape before any network request. Never log its value.
- `target.timezone`: valid IANA timezone string (parsed and validated by Go's `time.LoadLocation`).
- `target.weekday`: one of `Monday`, `Tuesday`, ..., `Sunday` (case-sensitive).
- `target.time`: valid 24-hour time in `HH:MM` format; both HH and MM are required integers.
- `target.same_day_threshold`: valid Go duration >= 1 hour.
- `poll.question`: nonempty string; after template substitution, max 300 characters.
- `poll.date_format`: valid Go time layout string (parsed by `time.Parse`).
- `poll.allow_multiselect`: boolean.
- `poll.answers`: array of 1–10 items.
  - `text`: nonempty string; after template substitution, max 55 characters.
  - `emoji`: optional string; if present, either valid Unicode emoji or custom emoji in format `<:name:id>` or `<a:name:id>`.

**JSON Schema file**: `schemas/user-config.schema.json` (documentation and editor support; not runtime-enforced).

### System configuration schema

File: `system-config.yaml`

All fields are required. No defaults. Unknown keys are rejected.

```yaml
logging:
  level: info

http:
  request_timeout: 10s
  max_retries: 2
  max_rate_limit_wait: 60s
```

**Validation rules**:
- `logging.level`: one of `debug`, `info`, `warn`, `error`.
- `http.request_timeout`: valid Go duration, 1s to 60s.
- `http.max_retries`: integer, 0 to 5.
- `http.max_rate_limit_wait`: valid Go duration, 1s to 120s.

**JSON Schema file**: `schemas/system-config.schema.json`.

## Exit codes

Documented in `README.md`:

| Exit Code | Meaning | Action for operator |
|---|---|---|
| 0 | Success. Poll created. | No action. Log checked for warnings (e.g., expiry deviation). |
| 1 | Unexpected internal error (panic, OOM, etc.). | Investigate logs. Likely a bug. |
| 2 | Usage or configuration error (invalid YAML, unknown keys, type mismatch, missing required fields). | Fix configuration. Do not retry until fixed. |
| 3 | Transport failure or ambiguous delivery (network error, timeout after sending, unable to reach Discord). | May retry on next scheduled run. Check connectivity. |
| 4 | Discord API rejection (HTTP 400–499, invalid token) or exhausted rate-limit retries. | Fix credentials or contact Discord support. Do not retry until resolved. |

## Logging strategy

- **Library**: `uber-go/zap` with production JSON output to stderr.
- **Level**: Configurable in system-config; default `info`.
- **No logfile**: output goes to systemd journal.
- **Credential redaction**: Never log webhook URL, token, or secret parts of error messages.
- **Dry-run**: Logs all the same operations (scheduling, validation, target selection) but skips the poll creation call. Logs from underlying HTTP client (e.g., connection debug messages) are suppressed during the GET webhook check.

### Log message examples

```json
{"level":"info","ts":1696412410.123,"msg":"Configuration loaded","config_file":"/etc/discord-poller/user-config.yaml"}
{"level":"info","ts":1696412410.124,"msg":"Webhook credential validated","webhook_id":"123456789"}
{"level":"info","ts":1696412410.125,"msg":"Target deadline selected","deadline":"2026-10-08T18:00:00+02:00","duration_hours":168}
{"level":"info","ts":1696412410.126,"msg":"Poll created","message_id":"999999999","poll_expiry":"2026-10-08T18:00:00+02:00"}
{"level":"warn","ts":1696412410.127,"msg":"Poll expiry outside intended window","intended":"2026-10-08T18:00:00+02:00","actual":"2026-10-08T18:00:15+02:00","deviation_seconds":15}
```

## Module structure

```
discord-poller/
├── cmd/
│   └── discord-poller/
│       └── main.go               # CLI entry point, flag parsing, composition
├── internal/
│   ├── config/
│   │   ├── config.go             # Config struct definitions
│   │   ├── loader.go             # YAML loading, unknown-key validation
│   │   ├── validation.go         # All validation logic
│   │   └── validation_test.go    # Tests for config
│   ├── schedule/
│   │   ├── schedule.go           # Deadline calculation and duration math
│   │   └── schedule_test.go      # Table-driven tests (test-first)
│   ├── template/
│   │   ├── template.go           # Placeholder resolution for question and answers
│   │   └── template_test.go      # Tests for template rendering
│   ├── discord/
│   │   ├── payload.go            # Payload construction and validation
│   │   ├── client.go             # HTTP client, webhook creation, rate-limit handling
│   │   ├── response.go           # Response parsing and expiry extraction
│   │   └── discord_test.go       # HTTP mock tests
│   └── logger/
│       └── logger.go             # Zap setup and credential redaction
├── schemas/
│   ├── user-config.schema.json
│   └── system-config.schema.json
├── docs/
│   ├── PLAN.md
│   ├── IMPLEMENTATION.md         # This file
│   └── examples/
│       ├── user-config.yaml
│       └── system-config.yaml
├── deployments/
│   ├── discord-poller.service
│   └── discord-poller.timer
├── go.mod
├── go.sum
├── README.md
└── Makefile (optional, only if command complexity grows)
```

## Test-first development (TDD) strategy

Write tests **first**, then implement. All core logic is covered by table-driven tests with no external dependencies. HTTP tests use `net/http/httptest`.

### Phase 1: Scheduling and configuration

**Test files to write first**:
- `internal/schedule/schedule_test.go` — table-driven tests for deadline selection
- `internal/config/validation_test.go` — config parsing and validation
- `internal/template/template_test.go` — placeholder substitution

**Expected test coverage**:
1. **schedule_test.go** (~40 test cases):
   - Every target weekday (Monday through Sunday)
   - Before cutoff (execution time < deadline - threshold)
   - At cutoff (execution time == deadline - threshold)
   - After cutoff (execution time > deadline - threshold)
   - Midnight boundaries
   - Month/year boundaries
   - DST transitions
   - Timezone conversions
   - Fractional thresholds (e.g., 1.5h)
   - Property: `deadline - 1h < nominal_expiry <= deadline`
   - Duration calculation: floor division
   - Subsecond precision

2. **validation_test.go** (~30 test cases):
   - Valid user and system configs
   - Missing required fields
   - Unknown keys (should fail)
   - Type mismatches (string where duration expected)
   - Out-of-range values (threshold < 1h, log level invalid)
   - Invalid timezone
   - Invalid weekday name
   - Invalid time format (HH:MM variants)
   - Invalid date format (malformed Go layout)
   - Answer text length validation (post-render)
   - Question length validation (post-render)
   - Too few/too many answers
   - Invalid emoji format
   - Missing or blank configured webhook URL

3. **template_test.go** (~20 test cases):
   - Single placeholder `{end_date}` in question
   - Placeholder in answer text
   - Multiple placeholders (future-proofing)
   - Unsupported placeholder (should fail)
   - Escaped braces using `{{` and `}}`; unmatched braces fail
   - Unicode strings and grapheme clusters
   - Empty template (should fail)
   - Rendered length reflects placeholder expansion; Discord limits are
     validated by the payload layer

### Phase 2: HTTP client and Discord integration

**Test files to write**:
- `internal/discord/discord_test.go` — HTTP mock tests

**Expected test coverage**:
1. **discord_test.go** (~25 test cases):
   - Successful poll creation (HTTP 200, with `wait=true`)
   - Successful poll creation with `poll.expiry` returned
   - Single-answer poll acceptance at payload level
   - HTTP 429 with valid `Retry-After` (should retry)
   - Missing or invalid `Retry-After` (fail without guessed delay)
   - HTTP 429 exceeding max wait budget (should fail with code 4)
   - Exhausted retries (should fail with code 4)
   - Context cancellation during rate-limit wait
   - HTTP 400 (invalid payload; fail with code 4)
   - HTTP 401 (invalid token; fail with code 4)
   - HTTP 5xx or network error (ambiguous; fail with code 3 and do not retry)
   - Expiry in returned response (extract and return to caller)
   - Missing expiry in response (report as unavailable)
   - Webhook URL/token redaction in errors
   - Webhook GET validation success
   - Webhook GET validation failure (invalid token)
   - 429 handling applies to GET and POST
   - Dry-run skips POST, allows GET

### Testing tools

- **Assertions**: `github.com/stretchr/testify/assert`
- **HTTP mocking**: `net/http/httptest`
- **Table-driven pattern**: `table []struct { name string; ... }`
- **Fakes/doubles**: Inject a clock (`time.Time` parameter) for reproducible deadlines; inject HTTP client for mocking

### Clock injection

Pass `now time.Time` (or a function `func() time.Time`) to schedule and config functions, not `time.Now()`. Tests set `now` to fixed values; production code uses `time.Now()`.

Example:
```go
func SelectDeadline(now time.Time, tz *time.Location, targetWeekday time.Weekday, targetTime time.Time, threshold time.Duration) (time.Time, error) {
  // ...
}
```

## Implementation phases

### Phase 1: Configuration, scheduling, templating (no network)

**Deliverable**: Parse both config files, select deadline, render template, validate all constraints. Pass all table-driven tests.

**Files**:
- `internal/config/config.go`
- `internal/config/loader.go`
- `internal/config/validation.go`
- `internal/config/validation_test.go` (TDD)
- `internal/schedule/schedule.go`
- `internal/schedule/schedule_test.go` (TDD)
- `internal/template/template.go`
- `internal/template/template_test.go` (TDD)
- `internal/logger/logger.go`
- `cmd/discord-poller/main.go` (basic composition, flag parsing)

**Success criteria**:
- All table-driven tests pass.
- Config with unknown keys is rejected.
- All validation rules enforced.
- Configuration can be loaded strictly, scheduled, rendered, and validated
  without any network requests.

### Phase 2: Discord HTTP client

**Deliverable**: Build poll payload, send to Discord webhook, handle rate limits and errors. Pass all HTTP mock tests.

**Files**:
- `internal/discord/payload.go`
- `internal/discord/client.go`
- `internal/discord/response.go`
- `internal/discord/discord_test.go` (TDD)

**Success criteria**:
- All HTTP mock tests pass.
- Retry-After header respected and parsed correctly.
- Ambiguous outcomes (POST sent but no response) fail with code 3, not retried.
- Returned expiry is available to the caller; no corrective repost is made.
- Webhook validation uses GET; poll creation uses POST with `wait=true`.

### Phase 3: Integration and CLI (implemented)

**Deliverable**: Compose all modules, implement error handling and exit codes,
and document CLI development and invocation.

**Files**:
- `cmd/discord-poller/main.go` (complete)
- `README.md` (development commands, build, CLI usage, exit codes)

**Success criteria**:
- Exit code contract enforced.
- Logs contain no secrets.
- Dry-run and real execution paths work.
- README documents development commands and CLI invocation.

### Phase 4: Deployment and CI (implemented)

**Deliverable**: Package examples, schemas, systemd deployment files, and
automated validation.

**Files**:
- `docs/examples/user-config.yaml`
- `docs/examples/system-config.yaml`
- `schemas/user-config.schema.json`
- `schemas/system-config.schema.json`
- `deployments/discord-poller.service`
- `deployments/discord-poller.timer`
- `.github/workflows/ci.yml`

**Success criteria**:
- README documents installation, deployment, configuration, and troubleshooting.
- Example configs satisfy JSON schemas.
- CI runs tests, vet, build, and schema/example consistency checks.

### CLI composition slice

Implement CLI tests first, then `cmd/discord-poller/main.go`. The CLI accepts
explicit paths for required user and system configuration, plus `--dry-run`.
It loads and validates both configurations, initializes the configured logger,
uses the webhook URL from user configuration,
selects the target deadline, renders question and answer templates, and
validates the payload before making a network request.

Both dry-run and normal execution validate the webhook with GET. Dry-run
requires the configured webhook credential and network access, and logs
the selected deadline, requested duration, and complete validated poll
payload, but never issues the poll-creation POST. Normal execution posts once,
then logs the message ID, requested duration, and returned expiry when
available. It reports an expiry outside the intended window as a warning and
never reposts.

Test usage and missing configuration paths; config and environment errors;
deadline, template, and payload validation; GET-only dry-run; successful GET
and POST; returned and missing expiry; expiry-window deviation; categorized
exit codes; cancellation; and absence of webhook secrets in logs and errors.

## Key implementation decisions

### YAML libraries

Candidate: `go.yaml.in/yaml/v3` or `gopkg.in/yaml.v3` (same package, aliases).
Rationale: Standard library support for `KnownFields` validation, active maintenance, no heavy dependencies.

Command to validate unknown keys in struct:
```go
var cfg UserConfig
decoder := yaml.NewDecoder(f)
decoder.KnownFields(true) // Reject unknown keys
err := decoder.Decode(&cfg)
```

### Emoji validation

- Unicode: Attempt to parse as a single rune or grapheme cluster. Accept if not an error. (Go's UTF-8 support is sufficient; full grapheme library not necessary for initial release.)
- Custom: Regex match `^<a?:[\w]+:\d+>$` for `<:name:id>` and `<a:name:id>`.

Implementation: One validation function in `internal/config/validation.go`.

### Duration parsing

Use `time.ParseDuration` for all duration strings (thresholds, timeouts). It accepts `3h`, `2h30m`, `90m`, etc.

### Timestamp formatting

Use Go's reference layout `02.01.2006` style. The user will supply the correct layout in config. No locale support.

### Webhook validation

Use Discord's nonmutating **Get Webhook with Token** endpoint. Docs: https://docs.discord.com/developers/resources/webhook#get-webhook-with-token

This does **not** prove a poll can be created (permissions, server limits, etc.), but it validates the URL and token format. Perform before dry-run or real execution. Never cache the result.

### HTTP client setup

Standard `net/http.Client` with explicit timeout. No Discord SDK.

Example:
```go
client := &http.Client{
  Timeout: systemConfig.HTTP.RequestTimeout,
}
```

### Retry logic for rate limits

1. Send request.
2. If HTTP 429, check `Retry-After` header (seconds, integer).
3. If invalid or missing, assume 60s (Discord's common value).
4. If remaining wait budget allows, sleep and retry up to `max_retries` times.
5. If budget exceeded or retries exhausted, fail with code 4.
6. Never retry non-429 errors or timeouts.

Pseudocode:
```
remaining_wait := max_rate_limit_wait
for attempt := 0; attempt <= max_retries; attempt++ {
  response := client.Do(request)
  if response.StatusCode != 429 {
    return response
  }
  delay := parseRetryAfter(response.Header.Get("Retry-After"))
  if delay > remaining_wait {
    return error "rate limit wait budget exceeded"
  }
  remaining_wait -= delay
  sleep(delay)
}
return error "retries exhausted"
```

### Dry-run behavior

1. Load both configs, including the webhook URL from the user config.
2. Validate all configuration.
3. Select deadline and render templates.
4. Perform webhook GET validation (validate credentials; network access is
   required).
5. Construct full payload (validated but not sent).
6. Log all the above, including the validated poll payload, without logging
   the webhook URL or token.
7. Exit with code 0 (success). Do not call POST.

### Credential redaction

Never include the webhook URL, token, or raw HTTP response body in client
errors or CLI logs. The Discord client returns categorized errors that expose
only the operation and HTTP status; the CLI logs no credential-derived data.

## Documentation

### README.md

- **Overview**: One sentence describing the tool.
- **Building**: `go build ./cmd/discord-poller` and resulting binary location.
- **Installation**: Copy binary, create systemd user/service directories, deploy configs.
- **Configuration**: Link to `docs/examples/`, brief explanation of user vs. system config.
- **Usage**: Basic invocation examples (`--config`, `--dry-run`).
- **Exit codes**: Full table.
- **Troubleshooting**: Common issues (invalid timezone, webhook token, network, rate limits).
- **Development**: Running tests (`go test ./...`), testing strategy.

### Example configs

Provide both `user-config.yaml` and `system-config.yaml` in `docs/examples/`, with inline comments.

### JSON Schemas

Both schemas in `schemas/` for editor validation and documentation. Validate offline with tools like `ajv` or `jsonschema` during CI if needed.

## CI and testing

### GitHub Actions workflow

Run on every push:
1. `go test ./...` — all tests must pass.
2. `go vet ./...` — no vet issues.
3. `go build ./cmd/discord-poller` — binary builds successfully.
4. (Optional) Schema validation against example configs.

No secrets required.

## Next steps

Perform the planned live test only after the user provides and authorizes use
of a webhook credential. Never commit or retain that credential in the
repository.

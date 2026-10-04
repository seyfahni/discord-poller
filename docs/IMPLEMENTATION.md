# Discord Poller — Detailed Implementation Plan

Status: ready for implementation.

## Overview

This document breaks down the implementation into concrete phases, testing strategy, file structure, and configuration schemas. All decisions from `docs/PLAN.md` are incorporated, along with clarifications on logging, configuration splitting, and exit codes.

## Configuration architecture

### User configuration vs. System configuration

**User configuration** (`user-config.yaml`): Poll content, target timing, timezone.  
**System configuration** (`system-config.yaml`): Logging, HTTP timeouts, retry behavior, credential sources.

Both are required. Users manage user-config; operators maintain system-config. Environment variables override both (e.g., `DISCORD_WEBHOOK_URL`, `LOG_LEVEL`).

### User configuration schema

File: `user-config.yaml`

All fields are required. No defaults. Unknown keys are rejected.

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

**Validation rules**:
- `webhook_url_env`: nonempty string, matches `^[A-Z_][A-Z0-9_]*$` (valid env var name).
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
   - Invalid webhook_url_env name

3. **template_test.go** (~20 test cases):
   - Single placeholder `{end_date}` in question
   - Placeholder in answer text
   - Multiple placeholders (future-proofing)
   - Unsupported placeholder (should fail)
   - Escaped braces (implementation TBD)
   - Unicode strings and grapheme clusters
   - Empty template (should fail)
   - Length validation after substitution

### Phase 2: HTTP client and Discord integration

**Test files to write**:
- `internal/discord/discord_test.go` — HTTP mock tests

**Expected test coverage**:
1. **discord_test.go** (~25 test cases):
   - Successful poll creation (HTTP 200, with `wait=true`)
   - Successful poll creation with `poll.expiry` returned
   - Missing `Retry-After` header, immediate success
   - HTTP 429 with `Retry-After: 5` (should retry)
   - HTTP 429 exceeding max wait budget (should fail with code 4)
   - Exhausted retries (should fail with code 4)
   - Unsupported HTTP 429 format (fall back to max wait exceeded)
   - HTTP 400 (invalid payload; fail with code 4)
   - HTTP 401 (invalid token; fail with code 4)
   - HTTP 500 (server error; fail with code 3)
   - Network timeout (fail with code 3)
   - Connection refused (fail with code 3)
   - POST sent but no response received (ambiguous; fail with code 3)
   - Expiry in returned response (extract and log)
   - Expiry outside intended window (log warning, succeed with code 0)
   - Credential redaction in error messages
   - Webhook GET validation success
   - Webhook GET validation failure (invalid token)
   - Rate limit applied to GET request
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
- `--dry-run` prints logs of deadline selection, template rendering, and payload validation without network calls.

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
- Expiry logged when returned; deviation reported.
- Dry-run calls GET webhook validation, but skips POST.

### Phase 3: Integration and CLI

**Deliverable**: Compose all modules, error handling, exit codes, systemd readiness.

**Files**:
- `cmd/discord-poller/main.go` (complete)
- `README.md` (build, install, usage, exit codes, troubleshooting)
- `docs/examples/user-config.yaml`
- `docs/examples/system-config.yaml`
- `schemas/user-config.schema.json`
- `schemas/system-config.schema.json`
- `deployments/discord-poller.service`
- `deployments/discord-poller.timer`

**Success criteria**:
- Exit code contract enforced.
- Logs contain no secrets.
- Dry-run and real execution paths work.
- README documents build commands, deployment, configuration.
- Example configs satisfy JSON schemas.

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

1. Load both configs and environment variables.
2. Validate all configuration.
3. Select deadline and render templates.
4. Perform webhook GET validation (validate credentials).
5. Construct full payload (validated but not sent).
6. Log all the above: "Configuration loaded", "Webhook credential validated", "Target deadline selected", "Payload constructed (dry-run, not sent)".
7. Exit with code 0 (success). Do not call POST.

### Credential redaction

Remove webhook URL and token from all error messages and logs:
- Before logging, replace the full URL with `***REDACTED***`.
- If an error message contains `token=...`, replace with `token=***REDACTED***`.
- Implement a sanitizer function in `internal/logger/logger.go`.

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

## Questions for clarification (none at this stage; all decisions incorporated)

The plan is ready for implementation. Proceed to Phase 1: configuration, scheduling, and templating logic.

## Next steps

1. Create the repository structure (directories, `go.mod`, initial files).
2. Write table-driven tests for `schedule_test.go` (first TDD phase).
3. Implement `schedule.go` to pass all tests.
4. Write and implement `config/validation_test.go`.
5. Write and implement `template/template_test.go`.
6. Continue to Phase 2 (HTTP client).


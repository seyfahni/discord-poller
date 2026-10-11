# Discord webhook poll with Thursday expiry

## Goal

Design a Bash script that posts a Discord poll through an incoming webhook. The poll duration is the whole number of hours from the time the script runs until the next Thursday at 18:00 in the host's local time.

## Requirements and decisions

- Use the machine's configured local timezone; do not hard-code a timezone or set a `TZ` override in the script. The scheduled environment must not override the host timezone.
- On Thursday before 17:00, target that day's 18:00. At or after 17:00, target the following Thursday.
- Round the elapsed time down to whole hours.
- Use a fixed poll question template and fixed answer choices; do not accept poll question or answer arguments.
- Set the question to the computed target end date in `DD.MM.YYYY` format (`DD.MM.YYYY` means two-digit day, two-digit month, four-digit year), for example `13.12.2026`.
- Use these fixed answers, with emoji separate from answer text: `👍` with text `Bin dabei!`, and `👎` with text `Passt leider nicht.`
- Read the webhook URL from `DISCORD_WEBHOOK_URL`, not a command-line argument, and never print it.

Discord's Poll Create Request accepts a duration in whole hours, up to 32 days, and the webhook Execute endpoint accepts a `poll` payload. A duration of at least one hour is required for this design.

## Proposed interface and dependencies

Implement one Bash script with no poll question, answer, content, or multiselect arguments. Do not include a message `content` field in the webhook payload, and always set `allow_multiselect` to `false`. Require Bash, GNU `date`, `curl`, and `jq`, and fail early with a clear message if a dependency or the webhook URL is missing.

Use `set -euo pipefail`, quote expansions, and use `jq` to construct the JSON payload rather than interpolating user input into JSON. Keep any temporary payload file private and clean it up with an exit trap.

The request body should use Discord's poll create-request structure:

```json
{
  "poll": {
    "question": { "text": "13.12.2026" },
    "answers": [
      { "poll_media": { "text": "Bin dabei!", "emoji": { "name": "👍" } } },
      { "poll_media": { "text": "Passt leider nicht.", "emoji": { "name": "👎" } } }
    ],
    "duration": 42,
    "allow_multiselect": false
  }
}
```

## Duration calculation

Capture the current epoch time once, then derive the current local calendar date, ISO weekday, and local time from that same instant. Find the next Thursday's local calendar date using weekday arithmetic. If today is Thursday, choose today only when the current local time is before 17:00; otherwise choose seven calendar days later. This cutoff guarantees at least one full hour until the 18:00 target, so no separate zero-duration rollover is needed.

Construct the target as 18:00 on that local date and convert it to epoch seconds. Then calculate:

```text
duration_hours = floor((target_epoch - now_epoch) / 3600)
```

Use elapsed seconds for the subtraction so daylight-saving changes are handled correctly; use local calendar arithmetic to keep the target at Thursday 18:00. Validate that the resulting duration is at least 1 and within Discord's supported range before posting.

Format the target date in the host's local timezone as `%d.%m.%Y` and use that exact string as the poll question.

## Request flow and failure handling

1. Validate the webhook URL and dependencies.
2. Calculate the duration and final target date, format the question as `DD.MM.YYYY`, and construct a payload with the fixed answer media, duration, and `allow_multiselect` set to `false`. Do not include message content.
3. POST the payload to the webhook Execute endpoint with `wait=true`, so Discord confirms message creation and returns its response.
4. Report success without printing the webhook URL. On an HTTP/API or local validation error, print a concise diagnostic to stderr and exit nonzero.

Do not automatically retry a failed POST: if the request reached Discord but its response was lost, a retry could create a duplicate poll.

## Verification plan

- Test target selection for Wednesday, Thursday before 17:00, exactly 17:00, and Thursday after 17:00.
- Test that Thursday before the cutoff has a duration of at least one hour, including exactly 17:00.
- Test duration flooring and local calendar arithmetic across a daylight-saving transition.
- Test that the question contains the target date formatted as `DD.MM.YYYY`.
- Test missing inputs/dependencies and Discord error responses; confirm failures do not print the webhook URL or trigger a retry.

## References

- [Discord Poll Resource](https://docs.discord.com/developers/resources/poll)
- [Discord Webhook Resource — Execute Webhook](https://docs.discord.com/developers/resources/webhook)

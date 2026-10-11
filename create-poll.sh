#!/bin/bash
set -euo pipefail
IFS=$'\n\t'

# Create a Discord poll that closes at the next scheduled Thursday at 18:00.

readonly MAX_POLL_DURATION_HOURS=768
TEMP_DIR=""

cleanup() {
    if [[ -n "$TEMP_DIR" && -d "$TEMP_DIR" ]]; then
        rm -rf -- "$TEMP_DIR"
    fi
}

trap cleanup EXIT

if (($# > 0)); then
    printf 'Error: this script does not accept arguments.\n' >&2
    exit 1
fi

for dependency in date curl jq; do
    if ! command -v "$dependency" >/dev/null 2>&1; then
        printf 'Error: required dependency not found: %s\n' "$dependency" >&2
        exit 1
    fi
done

date_version="$(date --version 2>/dev/null)" || {
    printf 'Error: GNU date is required.\n' >&2
    exit 1
}
if [[ "$date_version" != *"GNU coreutils"* ]]; then
    printf 'Error: GNU date is required.\n' >&2
    exit 1
fi

if [[ -z "${DISCORD_WEBHOOK_URL:-}" ]]; then
    printf 'Error: DISCORD_WEBHOOK_URL must be set and non-empty.\n' >&2
    exit 1
fi

if ! TEMP_DIR="$(mktemp -d)"; then
    printf 'Error: could not create a private temporary directory.\n' >&2
    exit 1
fi

readonly payload_file="${TEMP_DIR}/payload.json"
readonly response_file="${TEMP_DIR}/response.json"

now_epoch="$(date +%s)" || {
    printf 'Error: could not read the current time.\n' >&2
    exit 1
}
if [[ ! "$now_epoch" =~ ^[0-9]+$ ]]; then
    printf 'Error: date returned an invalid current time.\n' >&2
    exit 1
fi

current_parts="$(date -d "@${now_epoch}" '+%F %u %H%M')" || {
    printf 'Error: could not determine the local date and time.\n' >&2
    exit 1
}
IFS=' ' read -r current_date weekday current_hhmm <<< "$current_parts"
if [[ ! "$weekday" =~ ^[1-7]$ || ! "$current_hhmm" =~ ^[0-9]{4}$ ]]; then
    printf 'Error: date returned invalid local date or time data: parts="%s" weekday="%s" time="%s"\n' "$current_parts" "$weekday" "$current_hhmm" >&2
    exit 1
fi

days_until_thursday=$(((4 - weekday + 7) % 7))
if ((days_until_thursday == 0)) && ((10#$current_hhmm >= 1700)); then
    days_until_thursday=7
fi

target_date="$(date -d "${current_date} +${days_until_thursday} days" '+%F')" || {
    printf 'Error: could not calculate the target calendar date.\n' >&2
    exit 1
}
target_epoch="$(date -d "${target_date} 18:00:00" '+%s')" || {
    printf 'Error: could not calculate the target time.\n' >&2
    exit 1
}
question="$(date -d "@${target_epoch}" '+%d.%m.%Y')" || {
    printf 'Error: could not format the target date.\n' >&2
    exit 1
}

duration_hours=$(((target_epoch - now_epoch) / 3600))
if ((duration_hours < 1 || duration_hours > MAX_POLL_DURATION_HOURS)); then
    printf "Error: calculated poll duration (%d) is outside Discord's supported range.\n" "$duration_hours" >&2
    exit 1
fi

jq -n \
    --arg question "$question" \
    --argjson duration "$duration_hours" \
    '{
        poll: {
            question: {text: $question},
            answers: [
                {poll_media: {text: "Bin dabei!", emoji: {name: "👍"}}},
                {poll_media: {text: "Passt leider nicht.", emoji: {name: "👎"}}}
            ],
            duration: $duration,
            allow_multiselect: false
        }
    }' > "$payload_file" || {
        printf 'Error: could not construct the poll request.\n' >&2
        exit 1
    }

request_url="${DISCORD_WEBHOOK_URL}"
if [[ "$request_url" == *\?* ]]; then
    request_url+="&wait=true"
else
    request_url+="?wait=true"
fi

if ! http_status="$(curl --silent \
    --output "$response_file" \
    --write-out '%{http_code}' \
    --request POST \
    --header 'Content-Type: application/json' \
    --data-binary "@${payload_file}" \
    "$request_url" 2>/dev/null)"; then
    printf 'Error: failed to send the poll to Discord.\n' >&2
    exit 1
fi

if [[ ! "$http_status" =~ ^2[0-9][0-9]$ ]]; then
    printf 'Error: Discord rejected the poll request (HTTP %s).\n' "$http_status" >&2
    exit 1
fi

if ! jq --exit-status \
    'type == "object" and (.id | type == "string" and length > 0)' \
    "$response_file" >/dev/null 2>&1; then
    printf 'Error: Discord returned an invalid poll creation response.\n' >&2
    exit 1
fi

printf 'Poll created for %s (%s hours).\n' "$question" "$duration_hours"

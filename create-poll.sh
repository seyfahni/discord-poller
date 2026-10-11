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
    printf 'Error: this script does not accept arguments (received %d): %s\n' "$#" "$*" >&2
    exit 1
fi

for dependency in date curl jq; do
    if ! command -v "$dependency" >/dev/null 2>&1; then
        printf 'Error: required dependency not found: %s\n' "$dependency" >&2
        exit 1
    fi
done

if date_version="$(date --version 2>/dev/null)"; then
    :
else
    date_exit_status=$?
    printf 'Error: GNU date is required, but "date --version" failed (exit status %d).\n' \
        "$date_exit_status" >&2
    exit 1
fi
if [[ "$date_version" != *"GNU coreutils"* ]]; then
    printf 'Error: GNU date is required; detected date version: "%s".\n' "$date_version" >&2
    exit 1
fi

if [[ -z "${DISCORD_WEBHOOK_URL+x}" ]]; then
    printf 'Error: DISCORD_WEBHOOK_URL is unset; set it to a non-empty incoming webhook URL.\n' >&2
    exit 1
elif [[ -z "$DISCORD_WEBHOOK_URL" ]]; then
    printf 'Error: DISCORD_WEBHOOK_URL is set but empty; set it to a non-empty incoming webhook URL.\n' >&2
    exit 1
fi

if ! TEMP_DIR="$(mktemp -d 2>&1)"; then
    printf 'Error: could not create a private temporary directory: %s\n' "$TEMP_DIR" >&2
    TEMP_DIR=""
    exit 1
fi

readonly payload_file="${TEMP_DIR}/payload.json"
readonly response_file="${TEMP_DIR}/response.json"
readonly curl_error_file="${TEMP_DIR}/curl-error.txt"
readonly jq_error_file="${TEMP_DIR}/jq-error.txt"

if ! now_epoch="$(date +%s 2>&1)"; then
    printf 'Error: could not read the current epoch time: date output="%s".\n' "$now_epoch" >&2
    exit 1
fi
if [[ ! "$now_epoch" =~ ^[0-9]+$ ]]; then
    printf 'Error: date returned an invalid current epoch time: value="%s".\n' "$now_epoch" >&2
    exit 1
fi

if ! current_parts="$(date -d "@${now_epoch}" '+%F %u %H%M' 2>&1)"; then
    printf 'Error: could not determine local date and time from epoch "%s": date output="%s".\n' \
        "$now_epoch" "$current_parts" >&2
    exit 1
fi
IFS=' ' read -r current_date weekday current_hhmm <<< "$current_parts"
if [[ ! "$weekday" =~ ^[1-7]$ || ! "$current_hhmm" =~ ^[0-9]{4}$ ]]; then
    printf 'Error: date returned invalid local date or time data: parts="%s" date="%s" weekday="%s" time="%s".\n' \
        "$current_parts" "$current_date" "$weekday" "$current_hhmm" >&2
    exit 1
fi

days_until_thursday=$(((4 - weekday + 7) % 7))
if ((days_until_thursday == 0)) && ((10#$current_hhmm >= 1700)); then
    days_until_thursday=7
fi

if ! target_date="$(date -d "${current_date} +${days_until_thursday} days" '+%F' 2>&1)"; then
    printf 'Error: could not calculate target date from current date "%s" and offset %d days: date output="%s".\n' \
        "$current_date" "$days_until_thursday" "$target_date" >&2
    exit 1
fi
if ! target_epoch="$(date -d "${target_date} 18:00:00" '+%s' 2>&1)"; then
    printf 'Error: could not calculate target epoch from local target time "%s 18:00:00": date output="%s".\n' \
        "$target_date" "$target_epoch" >&2
    exit 1
fi
if [[ ! "$target_epoch" =~ ^[0-9]+$ ]]; then
    printf 'Error: date returned an invalid target epoch for "%s 18:00:00": value="%s".\n' \
        "$target_date" "$target_epoch" >&2
    exit 1
fi
if ! question="$(date -d "@${target_epoch}" '+%d.%m.%Y' 2>&1)"; then
    printf 'Error: could not format target date from epoch "%s": date output="%s".\n' \
        "$target_epoch" "$question" >&2
    exit 1
fi

duration_hours=$(((target_epoch - now_epoch) / 3600))
if ((duration_hours < 1 || duration_hours > MAX_POLL_DURATION_HOURS)); then
    printf "Error: calculated poll duration %d hours is outside Discord's supported range [1, %d]: now_epoch=%s target_epoch=%s target_date=%s target_time=18:00 local.\n" \
        "$duration_hours" "$MAX_POLL_DURATION_HOURS" "$now_epoch" "$target_epoch" "$target_date" >&2
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
    }' > "$payload_file" 2>"$jq_error_file" || {
        jq_error="$(<"$jq_error_file")"
        printf 'Error: jq could not construct the poll request (question="%s", duration=%d hours, payload_file="%s"): %s\n' \
            "$question" "$duration_hours" "$payload_file" "${jq_error:-no jq diagnostic}" >&2
        exit 1
    }

request_url="${DISCORD_WEBHOOK_URL}"
if [[ "$request_url" == *\?* ]]; then
    request_url+="&wait=true"
else
    request_url+="?wait=true"
fi

if http_status="$(curl --silent \
    --output "$response_file" \
    --write-out '%{http_code}' \
    --request POST \
    --header 'Content-Type: application/json' \
    --data-binary "@${payload_file}" \
    "$request_url" 2>"$curl_error_file")"; then
    :
else
    curl_exit_status=$?
    curl_error="$(<"$curl_error_file")"
    curl_error="${curl_error//"$request_url"/[webhook URL redacted]}"
    curl_error="${curl_error//"$DISCORD_WEBHOOK_URL"/[webhook URL redacted]}"
    printf 'Error: curl failed to send the poll (exit status %d, question="%s", duration=%d hours): %s\n' \
        "$curl_exit_status" "$question" "$duration_hours" "${curl_error:-no curl diagnostic}" >&2
    exit 1
fi

if [[ ! "$http_status" =~ ^2[0-9][0-9]$ ]]; then
    response_body="$(<"$response_file")"
    response_body="${response_body//"$request_url"/[webhook URL redacted]}"
    response_body="${response_body//"$DISCORD_WEBHOOK_URL"/[webhook URL redacted]}"
    printf 'Error: Discord rejected the poll (HTTP %s, question="%s", duration=%s hours); response body: %s\n' \
        "$http_status" "$question" "$duration_hours" "${response_body:-<empty>}" >&2
    exit 1
fi

if ! jq --exit-status \
    'type == "object" and (.id | type == "string" and length > 0)' \
    "$response_file" >/dev/null 2>&1; then
    response_body="$(<"$response_file")"
    response_body="${response_body//"$request_url"/[webhook URL redacted]}"
    response_body="${response_body//"$DISCORD_WEBHOOK_URL"/[webhook URL redacted]}"
    printf 'Error: Discord returned HTTP %s but the response is not a JSON object with a non-empty string id (question="%s"); response body: %s\n' \
        "$http_status" "$question" "${response_body:-<empty>}" >&2
    exit 1
fi

printf 'Poll created for %s (%s hours).\n' "$question" "$duration_hours"

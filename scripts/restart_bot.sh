#!/bin/bash
CD_PATH="/root/apps/super_cm_bot"
BOT_PATH="$CD_PATH/bot"
LOG_DIR="$CD_PATH/logs"
LOG_FILE="$LOG_DIR/restart_bot.log"
NOHUP_OUT="$LOG_DIR/nohup.out"
REPO="denis1011101/super_cm_bot"

log() {
    echo "$(date): $1" >> "$LOG_FILE"
}

# -f makes HTTP errors a non-zero exit instead of a body we would try to parse,
# -sS keeps it quiet but still reports transport failures on stderr.
gh_api() {
    curl -fsS -H "Accept: application/vnd.github.v3+json" "$1"
}

# Create log directory if it doesn't exist
if [ ! -d "$LOG_DIR" ]; then
    mkdir -p "$LOG_DIR"
fi

cd $CD_PATH

# Get information whether the bot is running
BOT_PIDS=$(pgrep -f "^$BOT_PATH$")
if [ -n "$BOT_PIDS" ]; then
    log "Bot is running with PID(s): $BOT_PIDS"
else
    log "Bot is not running. Restarting the bot."
    nohup $BOT_PATH &> $NOHUP_OUT &
    NEW_PID=$!
    log "Bot restarted with PID $NEW_PID"
fi

# Pin the commit up front. Everything below - the CI check and the download -
# refers to this exact SHA, so a push that lands mid-run cannot slip a binary
# past a check that was made against a different commit.
if ! COMMIT_JSON=$(gh_api "https://api.github.com/repos/$REPO/commits/main"); then
    log "Could not reach the commits API. Exiting script."
    exit 0
fi
MAIN_SHA=$(printf '%s' "$COMMIT_JSON" | jq -re '.sha' 2>/dev/null)
# The API silently returns no runs for an abbreviated SHA, so anything that is
# not a full 40-hex id has to stop the deploy rather than query with it.
if ! [[ "$MAIN_SHA" =~ ^[0-9a-f]{40}$ ]]; then
    log "Could not resolve main SHA. Exiting script."
    exit 0
fi
REMOTE_URL="https://raw.githubusercontent.com/$REPO/$MAIN_SHA/bot"

# Download once into a temp file: it is both what we hash and what we install,
# so the two cannot disagree.
TMP_BOT=$(mktemp "$CD_PATH/.bot.XXXXXX")
trap 'rm -f "$TMP_BOT"' EXIT

if ! curl -sfL -o "$TMP_BOT" "$REMOTE_URL"; then
    log "Failed to download bot at $MAIN_SHA. Exiting script."
    exit 0
fi

REMOTE_HASH=$(sha256sum "$TMP_BOT" | awk '{print $1}')

# Get hash of the current binary file
if [ -f "$BOT_PATH" ]; then
    LOCAL_HASH=$(sha256sum "$BOT_PATH" | awk '{print $1}')
else
    LOCAL_HASH=""
fi

# Check differences between the remote and local binary files
if [ "$REMOTE_HASH" == "$LOCAL_HASH" ]; then
    log "File bot has not changed. Exiting script."
    exit 0
else
    log "File bot has changed at $MAIN_SHA. Running script."
fi

# Require every workflow run for this exact commit to be finished and green.
# Filtering by branch instead would let an older success gate a newer binary,
# and a commit with no runs at all must block rather than pass.
if ! RUNS_JSON=$(gh_api "https://api.github.com/repos/$REPO/actions/runs?head_sha=$MAIN_SHA&per_page=100"); then
    log "Status: could not reach the runs API for $MAIN_SHA. Exiting script."
    exit 0
fi

# The whole admission decision lives in one jq -e expression, so the shell never
# compares values that a broken response could have left empty. Anything but a
# well-formed JSON object with a complete, green run list exits jq non-zero -
# parse errors, HTML, an empty body and a false verdict all land in the same
# branch, and the deploy stops.
ADMIT_FILTER='
    (type == "object")
    and (.total_count | type == "number")
    and (.workflow_runs | type == "array")
    and ((.workflow_runs | length) > 0)
    and (.total_count == (.workflow_runs | length))
    and (all(.workflow_runs[]; .status == "completed" and .conclusion == "success"))
'
if ! printf '%s' "$RUNS_JSON" | jq -e "$ADMIT_FILTER" > /dev/null 2>&1; then
    # Diagnostics only - this must never widen what was admitted above.
    REASON=$(printf '%s' "$RUNS_JSON" | jq -r '
        if (type != "object") or (.workflow_runs | type != "array") then "malformed response"
        elif (.workflow_runs | length) == 0 then "no workflow run"
        elif (.total_count != (.workflow_runs | length)) then "run list is paginated (\(.total_count) total)"
        elif any(.workflow_runs[]; .status != "completed") then "still in progress: " + ([.workflow_runs[] | select(.status != "completed") | .name] | join(", "))
        else "not green: " + ([.workflow_runs[] | "\(.name)=\(.conclusion)"] | join(", "))
        end' 2>/dev/null) || REASON=""
    log "Status: ${REASON:-unreadable response} for $MAIN_SHA. Exiting script."
    exit 0
fi

RUN_COUNT=$(printf '%s' "$RUNS_JSON" | jq -r '.workflow_runs | length')
log "Status: success for $MAIN_SHA ($RUN_COUNT run(s)). Proceeding with the script."

# Kill all processes related to the bot
BOT_PIDS=$(pgrep -f "^$BOT_PATH$")
if [ -n "$BOT_PIDS" ]; then
    pkill -f "^$BOT_PATH$" && log "Bot processes killed: $BOT_PIDS"
    sleep 1  # Give some time for processes to terminate
    # Verify if processes are still running
    BOT_PIDS=$(pgrep -f "^$BOT_PATH$")
    if [ -n "$BOT_PIDS" ]; then
        log "Failed to kill bot processes: $BOT_PIDS"
    else
        log "All bot processes successfully killed."
    fi
else
    log "No bot process found."
fi

# Install the binary we already downloaded and verified
mv -f "$TMP_BOT" "$BOT_PATH"
log "Bot file installed from $MAIN_SHA."

# Make the bot file executable
chmod +x $BOT_PATH
log "Bot file made executable."

# Start the bot in the background
nohup $BOT_PATH &> $NOHUP_OUT &
NEW_PID=$!
log "Bot restarted with PID $NEW_PID"

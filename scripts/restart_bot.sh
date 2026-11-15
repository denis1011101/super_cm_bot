#!/bin/bash
CD_PATH="${CD_PATH:-/root/apps/super_cm_bot}"
BOT_PATH="$CD_PATH/bot"
LOG_DIR="$CD_PATH/logs"
LOG_FILE="$LOG_DIR/restart_bot.log"
NOHUP_OUT="$LOG_DIR/nohup.out"
REPO="denis1011101/super_cm_bot"
API_URL="${API_URL:-https://api.github.com}"
DOWNLOAD_URL="${DOWNLOAD_URL:-https://github.com}"

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

# Deploy only published releases: the release workflow builds the binary from
# the tagged commit after a green CI run and attaches it as `bot` plus
# `bot.sha256`. A plain push to main therefore rolls nothing out.
if ! RELEASE_JSON=$(gh_api "$API_URL/repos/$REPO/releases/latest"); then
    log "Could not reach the releases API. Exiting script."
    exit 0
fi

TAG=$(printf '%s' "$RELEASE_JSON" | jq -re '.tag_name' 2>/dev/null)
# The tag ends up in a download URL, so anything but a plain vX.Y.Z stops here.
if ! [[ "$TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    log "Could not resolve the latest release tag. Exiting script."
    exit 0
fi

if ! printf '%s' "$RELEASE_JSON" | jq -e '[.assets[].name] | (index("bot") != null and index("bot.sha256") != null)' > /dev/null 2>&1; then
    log "Release $TAG has no bot binary attached yet. Exiting script."
    exit 0
fi

BASE_URL="$DOWNLOAD_URL/$REPO/releases/download/$TAG"

# The checksum is both the "has anything changed" check and the integrity
# check for what we are about to install.
TMP_SUM=$(mktemp "$CD_PATH/.bot.sha256.XXXXXX")
TMP_BOT=$(mktemp "$CD_PATH/.bot.XXXXXX")
trap 'rm -f "$TMP_BOT" "$TMP_SUM"' EXIT

if ! curl -sfL -o "$TMP_SUM" "$BASE_URL/bot.sha256"; then
    log "Failed to download bot.sha256 for $TAG. Exiting script."
    exit 0
fi
EXPECTED_HASH=$(awk '{print $1; exit}' "$TMP_SUM")
if ! [[ "$EXPECTED_HASH" =~ ^[0-9a-f]{64}$ ]]; then
    log "bot.sha256 for $TAG is malformed. Exiting script."
    exit 0
fi

# Get hash of the current binary file
if [ -f "$BOT_PATH" ]; then
    LOCAL_HASH=$(sha256sum "$BOT_PATH" | awk '{print $1}')
else
    LOCAL_HASH=""
fi

if [ "$EXPECTED_HASH" == "$LOCAL_HASH" ]; then
    log "File bot has not changed (release $TAG). Exiting script."
    exit 0
else
    log "Release $TAG differs from the installed bot. Running script."
fi

if ! curl -sfL -o "$TMP_BOT" "$BASE_URL/bot"; then
    log "Failed to download bot for $TAG. Exiting script."
    exit 0
fi

REMOTE_HASH=$(sha256sum "$TMP_BOT" | awk '{print $1}')
if [ "$REMOTE_HASH" != "$EXPECTED_HASH" ]; then
    log "Checksum mismatch for $TAG (expected $EXPECTED_HASH, got $REMOTE_HASH). Exiting script."
    exit 0
fi
log "Checksum verified for $TAG. Proceeding with the script."

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
log "Bot file installed from release $TAG."

# Make the bot file executable
chmod +x $BOT_PATH
log "Bot file made executable."

# Start the bot in the background
nohup $BOT_PATH &> $NOHUP_OUT &
NEW_PID=$!
log "Bot restarted with PID $NEW_PID"

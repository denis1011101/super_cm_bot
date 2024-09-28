#!/bin/bash
CD_PATH="/root/apps/super_cm_bot"
BOT_PATH="$CD_PATH/bot"
LOG_DIR="$CD_PATH/logs"
LOG_FILE="$LOG_DIR/restart_bot.log"
NOHUP_OUT="$LOG_DIR/nohup.out"
REMOTE_URL="https://github.com/denis1011101/super_cm_bot/raw/main/bot"

log() {
    echo "$(date): $1" >> "$LOG_FILE"
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

# Get hash of the binary file
REMOTE_HASH=$(curl -sL $REMOTE_URL | sha256sum | awk '{print $1}')

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
    log "File bot has changed. Running script."
fi

# Get the last status of workflow run
TEST_STATUS=$(curl -s -H "Accept: application/vnd.github.v3+json" \
                     https://api.github.com/repos/denis1011101/super_cm_bot/actions/runs | grep -m 1 '"conclusion"' | awk -F'"' '{print $4}')

# Check if the last workflow run was successful
if [ "$TEST_STATUS" == "success" ]; then
    log "Status: success. Proceeding with the script."
else
    log "Status: $TEST_STATUS. Exiting script."
    exit 0
fi

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

# Delete bot file
rm -f $BOT_PATH
log "Bot file deleted."

# Pull the latest binary file from the repository with curl
curl -L -o "$BOT_PATH" "$REMOTE_URL"
log "Bot file downloaded."

# Make the bot file executable
chmod +x $BOT_PATH
log "Bot file made executable."

# Start the bot in the background
nohup $BOT_PATH &> $NOHUP_OUT &
NEW_PID=$!
log "Bot restarted with PID $NEW_PID"

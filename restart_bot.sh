#!/bin/bash
CD_PATH="/root/apps/super_cum_bot"
BOT_PATH="$CD_PATH/bot"
LOG_DIR="$CD_PATH/logs"
LOG_FILE="$LOG_DIR/restart_bot.log"
NOHUP_OUT="$LOG_DIR/nohup.out"

log() {
    echo "$(date): $1" >> "$LOG_FILE"
}

# Создание директории для логов, если она не существует
if [ ! -d "$LOG_DIR" ]; then
    mkdir -p "$LOG_DIR"
fi

cd $CD_PATH

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

# Start the bot in the background
nohup $BOT_PATH &> $NOHUP_OUT &
NEW_PID=$!
log "Bot restarted with PID $NEW_PID"

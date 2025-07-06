# Super cm Bot

[![.github/workflows/go-ci.yml](https://github.com/denis1011101/super_cm_bot/actions/workflows/go-ci.yml/badge.svg)](https://github.com/denis1011101/super_cm_bot/actions/workflows/go-ci.yml)

Bot only for fun.

## Install

1. Clone the repo:
```sh
git clone https://github.com/denis1011101/super_cm_bot.git
cd super_cm_bot
```

2. Install dependencies
```sh
go get github.com/mattn/go-sqlite3
go mod download
```

3. Create a `.env` file based on `.env_example` and add the following:
```sh
cp .env_example .env
# Open .env and add BOT_TOKEN и SPECIFIC_CHAT_ID
```

## Run

1. Run the bot:
```sh
go run main.go
```

## Automation testing

1. Run tests:
```sh
go test -timeout=60s -count=1 ./tests
```

## Manual testing

1. Go to [@BotFather](https://t.me/BotFather)
2. Create test bot
3. Create test group
4. Add test bot to test group
5. Get group_id https://stackoverflow.com/questions/32423837/telegram-bot-how-to-get-a-group-chat-id
6. Put .env
7. Run bot

## Usage

The bot supports the following commands:
- `/pen`           - register and spin
- `/giga`          - choose a gigachat from members
- `/unhandsome`    - choose an unhandsome member
- `/toplength`     - show the top 10 by pen length
- `/topgiga`       - show the top 10 gigachats
- `/topunhandsome` - show the top 10 unhandsome members


## Administrative

To restart the bot every hours, add the following cron job:

```sh
0 * * * * cd /root/apps/super_cm_bot && scripts/restart_bot.sh
```

## License

[LICENSE](LICENSE)

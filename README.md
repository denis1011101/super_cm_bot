# Super cm Bot

[![.github/workflows/test.yml](https://github.com/denis1011101/super_cum_bot/actions/workflows/test.yml/badge.svg)](https://github.com/denis1011101/super_cum_bot/actions/workflows/test.yml)

Bot only for fun.

## Install

1. Clone the repo:
```sh
git clone https://github.com/denis1011101/super_cum_bot.git
cd super_cum_bot
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

## Testing

1. Run tests:
```sh
go test ./tests
```

## Usage

The bot supports the following commands:
- `/pen`           - register and spin
- `/giga`          - choose a gigachat from members
- `/unhandsome`    - choose an unhandsome member
- `/toplength`     - show the top 10 by pen length
- `/topgiga`       - show the top 10 gigachats
- `/topunhandsome` - show the top 10 unhandsome members


## Administrative

To restart the bot every 6 hours, add the following cron job:

```sh
0 */6 * * * cd /root/apps/super_cum_bot && scripts/restart_bot.sh
```

## License

[LICENSE](LICENSE)

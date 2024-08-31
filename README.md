# Super Cum Bot

Bot only for fun.

## Install

1. Clone the repo:
```sh
git clone https://github.com/denis1011101/super_cum_bot.git
cd super_cum_bot
```

2. Install dependencies
```sh
go mod download
go mod down
```

3. Create ф `.env` file based on `.env_example` and add the following:
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
go test
```

## Usage

The bot supports the following commands:
- `/pen`           - register and spin
- `/giga`          - choose a gigachat from members
- `/unhandsome`    - choose an unhandsome member
- `/toplength`     - show the top 10 by pen length
- `/topgiga`       - show the top 10 gigachats
- `/topunhandsome` - show the top 10 unhandsome members

## License

[LICENSE](LICENSE)
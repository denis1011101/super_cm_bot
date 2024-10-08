module github.com/denis1011101/super_cm_bot

go 1.22.2

require (
	github.com/go-telegram-bot-api/telegram-bot-api/v5 v5.5.1
	github.com/joho/godotenv v1.5.1
	github.com/mattn/go-sqlite3 v1.14.23
)

replace github.com/go-telegram-bot-api/telegram-bot-api/v5 => github.com/Keklil/telegram-bot-api/v5 v5.1.6

require gopkg.in/natefinch/lumberjack.v2 v2.2.1

require github.com/DATA-DOG/go-sqlmock v1.5.2

require github.com/jmoiron/sqlx v1.4.0

require (
	github.com/golang-migrate/migrate/v4 v4.18.1
	github.com/josestg/lazy v0.0.0-20230114190824-2bace4761b02
)

require (
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	go.uber.org/atomic v1.7.0 // indirect
)

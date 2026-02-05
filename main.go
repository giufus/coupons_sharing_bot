package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	if len(os.Args) > 1 {
		if err := handleCLI(os.Args[1:]); err != nil {
			log.Fatal(err)
		}
		return
	}

	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		log.Fatal("BOT_TOKEN is required")
	}
	path := os.Getenv("DB_PATH")
	if path == "" {
		path = "coupons.db"
	}

	db, err := openDB(path)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := migrate(db); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatalf("bot: %v", err)
	}
	log.Printf("Authorized as @%s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30
	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.CallbackQuery != nil {
			handleCallback(db, bot, update.CallbackQuery)
			continue
		}
		if update.Message == nil {
			continue
		}
		if update.Message.IsCommand() {
			handleCommand(db, bot, update.Message)
			continue
		}
		handleText(db, bot, update.Message)
	}
}

func handleCLI(args []string) error {
	if len(args) == 0 {
		return nil
	}
	switch args[0] {
	case "pin-set":
		return handlePinSet(args[1:])
	default:
		return errf("unknown command: %s", args[0])
	}
}

func handlePinSet(args []string) error {
	var dbPath string
	var pin string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--db":
			if i+1 >= len(args) {
				return errf("missing value for --db")
			}
			dbPath = args[i+1]
			i++
		case "--pin":
			if i+1 >= len(args) {
				return errf("missing value for --pin")
			}
			pin = args[i+1]
			i++
		default:
			return errf("unknown flag: %s", args[i])
		}
	}
	if strings.TrimSpace(dbPath) == "" {
		dbPath = "coupons.db"
	}
	if !isValidPin(pin) {
		return errf("PIN must be exactly 4 digits")
	}

	db, err := openDB(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := migrate(db); err != nil {
		return err
	}
	if err := setGlobalPin(db, pin); err != nil {
		return err
	}
	log.Printf("PIN hash stored in %s", dbPath)
	return nil
}

func errf(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}

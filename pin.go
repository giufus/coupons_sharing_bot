package main

import (
	"crypto/subtle"
	"database/sql"
	"errors"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var errPinNotSet = errors.New("pin not set")
var errPinMisconfigured = errors.New("pin misconfigured")

const pinConfigKey = "pin_hash"

func setGlobalPin(db *sql.DB, pin string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(pin), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = db.Exec(
		`INSERT INTO configurations (name, value, updated_at)
		 VALUES (?, ?, ?)
		 ON CONFLICT(name) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		pinConfigKey, string(hash), now,
	)
	return err
}

func verifyGlobalPin(db *sql.DB, pin string) error {
	if hash, ok := envPinHash(); ok {
		return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pin))
	}
	if plain, ok := envPin(); ok {
		// Constant-time compare to avoid trivial timing leaks.
		if subtle.ConstantTimeCompare([]byte(pin), []byte(plain)) != 1 {
			return errors.New("invalid pin")
		}
		return nil
	}

	hash, err := getConfigValue(db, pinConfigKey)
	if err != nil {
		if errors.Is(err, errNotFound) {
			return errPinNotSet
		}
		return err
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pin))
}

func hasGlobalPin(db *sql.DB) (bool, error) {
	_, err := getConfigValue(db, pinConfigKey)
	if err != nil {
		if errors.Is(err, errNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func isGlobalPinConfigured(db *sql.DB) (bool, error) {
	if _, ok := envPinHash(); ok {
		return true, nil
	}
	if _, ok := envPin(); ok {
		return true, nil
	}
	return hasGlobalPin(db)
}

func validatePinConfig() error {
	if _, ok := envPin(); ok {
		if _, ok2 := envPinHash(); ok2 {
			return errPinMisconfigured
		}
	}
	if plain, ok := envPin(); ok {
		if !isValidPin(plain) {
			return errPinMisconfigured
		}
	}
	return nil
}

func envPin() (string, bool) {
	v := strings.TrimSpace(os.Getenv("BOT_PIN"))
	if v == "" {
		return "", false
	}
	return v, true
}

func envPinHash() (string, bool) {
	v := strings.TrimSpace(os.Getenv("BOT_PIN_HASH"))
	if v == "" {
		return "", false
	}
	return v, true
}

func getConfigValue(db *sql.DB, name string) (string, error) {
	row := db.QueryRow(`SELECT value FROM configurations WHERE name = ?`, name)
	var v string
	if err := row.Scan(&v); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", errNotFound
		}
		return "", err
	}
	return v, nil
}

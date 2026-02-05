package main

import (
	"database/sql"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var errPinNotSet = errors.New("pin not set")

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

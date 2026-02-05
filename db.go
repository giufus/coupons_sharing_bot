package main

import (
	"database/sql"
	"fmt"
)

func openDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func migrate(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS coupons (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			value TEXT NOT NULL,
			platform TEXT NOT NULL,
			year INTEGER NOT NULL,
			month INTEGER,
			user_id INTEGER NOT NULL,
			username TEXT,
			tags TEXT NOT NULL DEFAULT '[]',
			likes INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			CHECK (month IS NULL OR (month >= 1 AND month <= 12))
		);`,
		`CREATE TABLE IF NOT EXISTS configurations (
			name TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS coupon_stars (
			coupon_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			username TEXT,
			starred_at TEXT NOT NULL,
			PRIMARY KEY (coupon_id, user_id),
			FOREIGN KEY (coupon_id) REFERENCES coupons(id) ON DELETE CASCADE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_coupons_platform ON coupons(platform);`,
		`CREATE INDEX IF NOT EXISTS idx_coupons_year ON coupons(year);`,
		`CREATE INDEX IF NOT EXISTS idx_coupons_month ON coupons(month);`,
		`CREATE INDEX IF NOT EXISTS idx_coupons_platform_year_month ON coupons(platform, year, month);`,
		`CREATE INDEX IF NOT EXISTS idx_coupons_user_id ON coupons(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_coupon_stars_user_id ON coupon_stars(user_id);`,
	}

	for i, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("migration %d failed: %w", i+1, err)
		}
	}
	return nil
}

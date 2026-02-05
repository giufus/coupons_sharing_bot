package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

var errNotFound = errors.New("not found")
var errForbidden = errors.New("forbidden")

func createCoupon(db *sql.DB, c Coupon) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	tagsJSON, err := json.Marshal(c.Tags)
	if err != nil {
		return 0, err
	}
	res, err := db.Exec(
		`INSERT INTO coupons (value, platform, year, month, user_id, username, tags, likes, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, 0, ?, ?)`,
		c.Value, c.Platform, c.Year, c.Month, c.UserID, c.Username, string(tagsJSON), now, now,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func getCouponByID(db *sql.DB, id int64) (Coupon, error) {
	row := db.QueryRow(`SELECT id, value, platform, year, month, user_id, username, tags, likes FROM coupons WHERE id = ?`, id)
	var c Coupon
	var tagsJSON string
	var month sql.NullInt64
	if err := row.Scan(&c.ID, &c.Value, &c.Platform, &c.Year, &month, &c.UserID, &c.Username, &tagsJSON, &c.Likes); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Coupon{}, errNotFound
		}
		return Coupon{}, err
	}
	if month.Valid {
		m := int(month.Int64)
		c.Month = &m
	}
	_ = json.Unmarshal([]byte(tagsJSON), &c.Tags)
	return c, nil
}

func updateCoupon(db *sql.DB, userID int64, id int64, patch CouponPatch) error {
	c, err := getCouponByID(db, id)
	if err != nil {
		return err
	}
	if c.UserID != userID {
		return errForbidden
	}

	if patch.Value != nil {
		c.Value = *patch.Value
	}
	if patch.Platform != nil {
		c.Platform = *patch.Platform
	}
	if patch.Year != nil {
		c.Year = *patch.Year
	}
	if patch.MonthSet {
		c.Month = patch.Month
	}
	if patch.Tags != nil {
		c.Tags = patch.Tags
	}

	if len(c.Value) == 0 || len(c.Platform) == 0 || c.Year == 0 {
		return errors.New("value, platform, and year are required")
	}

	tagsJSON, err := json.Marshal(c.Tags)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)

	_, err = db.Exec(
		`UPDATE coupons SET value = ?, platform = ?, year = ?, month = ?, tags = ?, updated_at = ? WHERE id = ?`,
		c.Value, c.Platform, c.Year, c.Month, string(tagsJSON), now, c.ID,
	)
	return err
}

func deleteCoupon(db *sql.DB, userID int64, id int64) error {
	c, err := getCouponByID(db, id)
	if err != nil {
		return err
	}
	if c.UserID != userID {
		return errForbidden
	}
	_, err = db.Exec(`DELETE FROM coupons WHERE id = ?`, id)
	return err
}

func searchCoupons(db *sql.DB, platform *string, year int, month *int, limit int) ([]Coupon, error) {
	query := `SELECT id, value, platform, year, month, user_id, username, tags, likes FROM coupons WHERE year = ?`
	args := []any{year}
	if platform != nil && *platform != "" {
		query += ` AND platform LIKE ? COLLATE NOCASE`
		args = append(args, "%"+*platform+"%")
	}
	if month != nil {
		query += ` AND month = ?`
		args = append(args, *month)
	}
	query += ` ORDER BY likes DESC, id DESC LIMIT ?`
	args = append(args, limit)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var coupons []Coupon
	for rows.Next() {
		var c Coupon
		var tagsJSON string
		var monthVal sql.NullInt64
		if err := rows.Scan(&c.ID, &c.Value, &c.Platform, &c.Year, &monthVal, &c.UserID, &c.Username, &tagsJSON, &c.Likes); err != nil {
			return nil, err
		}
		if monthVal.Valid {
			m := int(monthVal.Int64)
			c.Month = &m
		}
		_ = json.Unmarshal([]byte(tagsJSON), &c.Tags)
		coupons = append(coupons, c)
	}
	return coupons, rows.Err()
}

func starCoupon(db *sql.DB, couponID int64, userID int64, username string) (bool, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	res, err := tx.Exec(`INSERT OR IGNORE INTO coupon_stars (coupon_id, user_id, username, starred_at) VALUES (?, ?, ?, ?)`, couponID, userID, username, now)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	if affected == 0 {
		return false, tx.Commit()
	}
	if _, err := tx.Exec(`UPDATE coupons SET likes = likes + 1 WHERE id = ?`, couponID); err != nil {
		return false, err
	}
	return true, tx.Commit()
}

func unstarCoupon(db *sql.DB, couponID int64, userID int64) (bool, error) {
	tx, err := db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	res, err := tx.Exec(`DELETE FROM coupon_stars WHERE coupon_id = ? AND user_id = ?`, couponID, userID)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	if affected == 0 {
		return false, tx.Commit()
	}
	if _, err := tx.Exec(`UPDATE coupons SET likes = CASE WHEN likes > 0 THEN likes - 1 ELSE 0 END WHERE id = ?`, couponID); err != nil {
		return false, err
	}
	return true, tx.Commit()
}

func listStars(db *sql.DB, couponID int64, limit int) ([]Star, error) {
	rows, err := db.Query(`SELECT user_id, username FROM coupon_stars WHERE coupon_id = ? ORDER BY starred_at DESC LIMIT ?`, couponID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var stars []Star
	for rows.Next() {
		var s Star
		if err := rows.Scan(&s.UserID, &s.Username); err != nil {
			return nil, err
		}
		stars = append(stars, s)
	}
	return stars, rows.Err()
}

// CouponPatch enables partial updates.
type CouponPatch struct {
	Value    *string
	Platform *string
	Year     *int
	Month    *int
	MonthSet bool
	Tags     []string
}

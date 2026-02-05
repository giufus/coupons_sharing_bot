package main

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func newTestDB(t *testing.T) *DBHandle {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := openDB(path)
	if err != nil {
		t.Fatalf("openDB: %v", err)
	}
	if err := migrate(db); err != nil {
		_ = db.Close()
		t.Fatalf("migrate: %v", err)
	}
	return &DBHandle{db: db}
}

type DBHandle struct {
	db *sql.DB
}

func (h *DBHandle) Close() {
	_ = h.db.Close()
}

func TestCouponCRUDAndOwnership(t *testing.T) {
	h := newTestDB(t)
	defer h.Close()

	id, err := createCoupon(h.db, Coupon{
		Value:    "SAVE20",
		Platform: "Amazon",
		Year:     2025,
		UserID:   1,
		Username: "alice",
		Tags:     []string{"electronics"},
	})
	if err != nil {
		t.Fatalf("createCoupon: %v", err)
	}

	c, err := getCouponByID(h.db, id)
	if err != nil {
		t.Fatalf("getCouponByID: %v", err)
	}
	if c.Value != "SAVE20" || c.Platform != "Amazon" || c.Year != 2025 || c.UserID != 1 {
		t.Fatalf("unexpected coupon: %+v", c)
	}

	// Ownership check
	patch := CouponPatch{Value: strPtr("SAVE30")}
	if err := updateCoupon(h.db, 2, id, patch); err != errForbidden {
		t.Fatalf("expected errForbidden, got %v", err)
	}

	if err := updateCoupon(h.db, 1, id, patch); err != nil {
		t.Fatalf("updateCoupon: %v", err)
	}
	c, _ = getCouponByID(h.db, id)
	if c.Value != "SAVE30" {
		t.Fatalf("expected updated value, got %s", c.Value)
	}

	if err := deleteCoupon(h.db, 2, id); err != errForbidden {
		t.Fatalf("expected errForbidden on delete, got %v", err)
	}
	if err := deleteCoupon(h.db, 1, id); err != nil {
		t.Fatalf("deleteCoupon: %v", err)
	}
	if _, err := getCouponByID(h.db, id); err != errNotFound {
		t.Fatalf("expected errNotFound after delete, got %v", err)
	}
}

func TestSearchPlatformOptionalAndPartial(t *testing.T) {
	h := newTestDB(t)
	defer h.Close()

	_, _ = createCoupon(h.db, Coupon{Value: "SAVE", Platform: "Amazon", Year: 2025, UserID: 1})
	_, _ = createCoupon(h.db, Coupon{Value: "OFF", Platform: "eBay", Year: 2025, UserID: 2})
	_, _ = createCoupon(h.db, Coupon{Value: "X", Platform: "Amazon", Year: 2024, UserID: 3})

	// Platform optional (year only)
	res, err := searchCoupons(h.db, nil, 2025, nil, 10)
	if err != nil {
		t.Fatalf("searchCoupons: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("expected 2 results, got %d", len(res))
	}

	// Partial + case-insensitive
	plat := "ama"
	res, err = searchCoupons(h.db, &plat, 2025, nil, 10)
	if err != nil {
		t.Fatalf("searchCoupons: %v", err)
	}
	if len(res) != 1 || res[0].Platform != "Amazon" {
		t.Fatalf("expected Amazon result, got %+v", res)
	}
}

func TestStarUnstar(t *testing.T) {
	h := newTestDB(t)
	defer h.Close()

	id, _ := createCoupon(h.db, Coupon{Value: "SAVE", Platform: "Amazon", Year: 2025, UserID: 1})

	added, err := starCoupon(h.db, id, 2, "bob")
	if err != nil || !added {
		t.Fatalf("starCoupon expected added, err=%v", err)
	}
	c, _ := getCouponByID(h.db, id)
	if c.Likes != 1 {
		t.Fatalf("expected likes=1, got %d", c.Likes)
	}

	added, err = starCoupon(h.db, id, 2, "bob")
	if err != nil || added {
		t.Fatalf("expected duplicate star ignored, added=%v err=%v", added, err)
	}

	removed, err := unstarCoupon(h.db, id, 2)
	if err != nil || !removed {
		t.Fatalf("unstar expected removed, err=%v", err)
	}
	c, _ = getCouponByID(h.db, id)
	if c.Likes != 0 {
		t.Fatalf("expected likes=0, got %d", c.Likes)
	}
}

func strPtr(s string) *string { return &s }

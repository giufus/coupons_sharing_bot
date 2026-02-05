package main

type Coupon struct {
	ID       int64
	Value    string
	Platform string
	Year     int
	Month    *int
	UserID   int64
	Username string
	Tags     []string
	Likes    int
}

type Star struct {
	UserID   int64
	Username string
}

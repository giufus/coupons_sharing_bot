package main

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const maxSearchResults = 20
const maxStarList = 10

func handleCommand(db *sql.DB, bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	cmd := msg.Command()
	args := msg.CommandArguments()

	switch cmd {
	case "start", "help":
		reply(bot, msg, helpText())
	case "add":
		handleAdd(db, bot, msg, args)
	case "update":
		handleUpdate(db, bot, msg, args)
	case "delete":
		handleDelete(db, bot, msg, args)
	case "search":
		handleSearch(db, bot, msg, args)
	case "star":
		handleStar(db, bot, msg, args)
	case "unstar":
		handleUnstar(db, bot, msg, args)
	default:
		reply(bot, msg, "Unknown command. Use /help for usage.")
	}
}

func helpText() string {
	return strings.Join([]string{
		"Coupon Sharing Bot commands:",
		"/add value=... platform=... year=YYYY month=MM tags=tag1,tag2",
		"/update id=ID [value=...] [platform=...] [year=YYYY] [month=MM|month=] [tags=tag1,tag2]",
		"/delete id=ID",
		"/search year=YYYY [platform=...] [month=MM]",
		"/star id=ID",
		"/unstar id=ID",
		"Notes:",
		"- Use quotes for values with spaces, e.g. value=\"SUMMER 20\"",
		"- search platform is case-insensitive and supports partial match",
		"- month= clears the month (for yearly coupons)",
	}, "\n")
}

func handleAdd(db *sql.DB, bot *tgbotapi.BotAPI, msg *tgbotapi.Message, args string) {
	kv, err := parseKeyValues(args)
	if err != nil {
		reply(bot, msg, "Invalid arguments: "+err.Error())
		return
	}
	value := kv["value"]
	platform := kv["platform"]
	yearStr := kv["year"]
	monthStr := kv["month"]
	tagsStr := kv["tags"]

	if value == "" || platform == "" || yearStr == "" {
		reply(bot, msg, "value, platform, and year are required")
		return
	}
	year, err := strconv.Atoi(yearStr)
	if err != nil || year <= 0 {
		reply(bot, msg, "year must be a valid number")
		return
	}
	var month *int
	if monthStr != "" {
		m, err := strconv.Atoi(monthStr)
		if err != nil || m < 1 || m > 12 {
			reply(bot, msg, "month must be 1-12")
			return
		}
		month = &m
	}

	id, err := createCoupon(db, Coupon{
		Value:    value,
		Platform: platform,
		Year:     year,
		Month:    month,
		UserID:   msg.From.ID,
		Username: msg.From.UserName,
		Tags:     parseTags(tagsStr),
	})
	if err != nil {
		reply(bot, msg, "Failed to add coupon: "+err.Error())
		return
	}
	reply(bot, msg, fmt.Sprintf("Coupon added with ID %d", id))
}

func handleUpdate(db *sql.DB, bot *tgbotapi.BotAPI, msg *tgbotapi.Message, args string) {
	kv, err := parseKeyValues(args)
	if err != nil {
		reply(bot, msg, "Invalid arguments: "+err.Error())
		return
	}
	idStr := kv["id"]
	if idStr == "" {
		reply(bot, msg, "id is required")
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		reply(bot, msg, "id must be a number")
		return
	}

	var patch CouponPatch
	if v, ok := kv["value"]; ok {
		patch.Value = &v
	}
	if v, ok := kv["platform"]; ok {
		patch.Platform = &v
	}
	if v, ok := kv["year"]; ok {
		val, err := strconv.Atoi(v)
		if err != nil || val <= 0 {
			reply(bot, msg, "year must be a valid number")
			return
		}
		patch.Year = &val
	}
	if v, ok := kv["month"]; ok {
		patch.MonthSet = true
		if v == "" {
			patch.Month = nil
		} else {
			val, err := strconv.Atoi(v)
			if err != nil || val < 1 || val > 12 {
				reply(bot, msg, "month must be 1-12")
				return
			}
			patch.Month = &val
		}
	}
	if v, ok := kv["tags"]; ok {
		patch.Tags = parseTags(v)
	}

	if err := updateCoupon(db, msg.From.ID, id, patch); err != nil {
		switch err {
		case errNotFound:
			reply(bot, msg, "Coupon not found")
		case errForbidden:
			reply(bot, msg, "You can only edit your own coupons")
		default:
			reply(bot, msg, "Failed to update coupon: "+err.Error())
		}
		return
	}
	reply(bot, msg, "Coupon updated")
}

func handleDelete(db *sql.DB, bot *tgbotapi.BotAPI, msg *tgbotapi.Message, args string) {
	kv, err := parseKeyValues(args)
	if err != nil {
		reply(bot, msg, "Invalid arguments: "+err.Error())
		return
	}
	idStr := kv["id"]
	if idStr == "" {
		reply(bot, msg, "id is required")
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		reply(bot, msg, "id must be a number")
		return
	}

	if err := deleteCoupon(db, msg.From.ID, id); err != nil {
		switch err {
		case errNotFound:
			reply(bot, msg, "Coupon not found")
		case errForbidden:
			reply(bot, msg, "You can only delete your own coupons")
		default:
			reply(bot, msg, "Failed to delete coupon: "+err.Error())
		}
		return
	}
	reply(bot, msg, "Coupon deleted")
}

func handleSearch(db *sql.DB, bot *tgbotapi.BotAPI, msg *tgbotapi.Message, args string) {
	kv, err := parseKeyValues(args)
	if err != nil {
		reply(bot, msg, "Invalid arguments: "+err.Error())
		return
	}
	platform := kv["platform"]
	yearStr := kv["year"]
	monthStr := kv["month"]

	if yearStr == "" {
		reply(bot, msg, "year is required")
		return
	}
	year, err := strconv.Atoi(yearStr)
	if err != nil || year <= 0 {
		reply(bot, msg, "year must be a valid number")
		return
	}

	var month *int
	if monthStr != "" {
		m, err := strconv.Atoi(monthStr)
		if err != nil || m < 1 || m > 12 {
			reply(bot, msg, "month must be 1-12")
			return
		}
		month = &m
	}

	var platformPtr *string
	if platform != "" {
		platformPtr = &platform
	}
	coupons, err := searchCoupons(db, platformPtr, year, month, maxSearchResults)
	if err != nil {
		reply(bot, msg, "Search failed: "+err.Error())
		return
	}
	if len(coupons) == 0 {
		reply(bot, msg, "No coupons found")
		return
	}

	var b strings.Builder
	b.WriteString("Results:\n")
	for _, c := range coupons {
		stars, _ := listStars(db, c.ID, maxStarList)
		starNames := formatStarUsers(stars)
		b.WriteString(fmt.Sprintf("#%d | %s | %s | %d-%s | likes:%d | by @%s\n", c.ID, c.Value, c.Platform, c.Year, formatMonth(c.Month), c.Likes, safeUsername(c.Username)))
		if len(c.Tags) > 0 {
			b.WriteString("tags: " + strings.Join(c.Tags, ",") + "\n")
		}
		if starNames != "" {
			b.WriteString("stars: " + starNames + "\n")
		}
		b.WriteString("\n")
	}
	reply(bot, msg, b.String())
}

func handleStar(db *sql.DB, bot *tgbotapi.BotAPI, msg *tgbotapi.Message, args string) {
	id, err := parseIDArg(args)
	if err != nil {
		reply(bot, msg, err.Error())
		return
	}
	_, err = getCouponByID(db, id)
	if err != nil {
		reply(bot, msg, "Coupon not found")
		return
	}

	added, err := starCoupon(db, id, msg.From.ID, msg.From.UserName)
	if err != nil {
		reply(bot, msg, "Failed to star: "+err.Error())
		return
	}
	if !added {
		reply(bot, msg, "You already starred this coupon")
		return
	}
	reply(bot, msg, "Star added")
}

func handleUnstar(db *sql.DB, bot *tgbotapi.BotAPI, msg *tgbotapi.Message, args string) {
	id, err := parseIDArg(args)
	if err != nil {
		reply(bot, msg, err.Error())
		return
	}
	removed, err := unstarCoupon(db, id, msg.From.ID)
	if err != nil {
		reply(bot, msg, "Failed to unstar: "+err.Error())
		return
	}
	if !removed {
		reply(bot, msg, "You have not starred this coupon")
		return
	}
	reply(bot, msg, "Star removed")
}

func parseIDArg(args string) (int64, error) {
	kv, err := parseKeyValues(args)
	if err != nil {
		return 0, fmt.Errorf("Invalid arguments: %s", err)
	}
	idStr := kv["id"]
	if idStr == "" {
		return 0, fmt.Errorf("id is required")
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("id must be a number")
	}
	return id, nil
}

func formatStarUsers(stars []Star) string {
	if len(stars) == 0 {
		return ""
	}
	parts := make([]string, 0, len(stars))
	for _, s := range stars {
		name := safeUsername(s.Username)
		if name == "" {
			parts = append(parts, fmt.Sprintf("uid:%d", s.UserID))
			continue
		}
		parts = append(parts, "@"+name)
	}
	return strings.Join(parts, ",")
}

func safeUsername(u string) string {
	return strings.TrimPrefix(u, "@")
}

func reply(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, text string) {
	m := tgbotapi.NewMessage(msg.Chat.ID, text)
	m.ReplyToMessageID = msg.MessageID
	_, _ = bot.Send(m)
}

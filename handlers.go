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
		sendMainMenu(bot, msg.Chat.ID)
		reply(bot, msg, helpText())
	case "pin":
		handlePin(db, bot, msg, args)
	case "add":
		if strings.TrimSpace(args) == "" {
			startAddFlow(bot, msg.Chat.ID, msg.From.ID)
			replyWithKeyboard(bot, msg, "Add flow started. Please enter PIN:", flowKeyboard(false))
			return
		}
		handleAdd(db, bot, msg, args)
	case "update":
		handleUpdate(db, bot, msg, args)
	case "delete":
		handleDelete(db, bot, msg, args)
	case "search":
		if strings.TrimSpace(args) == "" {
			startSearchFlow(bot, msg.Chat.ID, msg.From.ID)
			replyWithKeyboard(bot, msg, "Search flow started. Please enter PIN:", flowKeyboard(false))
			return
		}
		handleSearch(db, bot, msg, args)
	case "star":
		handleStar(db, bot, msg, args)
	case "unstar":
		handleUnstar(db, bot, msg, args)
	case "cancel":
		flows.clear(msg.Chat.ID, msg.From.ID)
		reply(bot, msg, "Canceled.")
	default:
		reply(bot, msg, "Unknown command. Use /help for usage.")
	}
}

func helpText() string {
	return strings.Join([]string{
		"Coupon Sharing Bot commands:",
		"/pin 1234",
		"/add pin=1234 value=... platform=... year=YYYY month=MM tags=tag1,tag2",
		"/update id=ID [value=...] [platform=...] [year=YYYY] [month=MM|month=] [tags=tag1,tag2]",
		"/delete id=ID",
		"/search pin=1234 year=YYYY [platform=...] [month=MM]",
		"/star id=ID",
		"/unstar id=ID",
		"/cancel",
		"Notes:",
		"- Use quotes for values with spaces, e.g. value=\"SUMMER 20\"",
		"- search platform is case-insensitive and supports partial match",
		"- you must provide the global PIN before add/search",
		"- month= clears the month (for yearly coupons)",
		"- You can use the on-screen buttons for Add/Search/Help/Cancel",
	}, "\n")
}

func handleAdd(db *sql.DB, bot *tgbotapi.BotAPI, msg *tgbotapi.Message, args string) {
	kv, err := parseKeyValues(args)
	if err != nil {
		reply(bot, msg, "Invalid arguments: "+err.Error())
		return
	}
	if !verifyPinFromKV(db, bot, msg, kv) {
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
	if !verifyPinFromKV(db, bot, msg, kv) {
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

	sendSearchResults(bot, msg, coupons)
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

func handleText(db *sql.DB, bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	text := strings.TrimSpace(msg.Text)
	switch strings.ToLower(text) {
	case "add coupon":
		startAddFlow(bot, msg.Chat.ID, msg.From.ID)
		replyWithKeyboard(bot, msg, "Add flow started. Please enter PIN:", flowKeyboard(false))
		return
	case "search":
		startSearchFlow(bot, msg.Chat.ID, msg.From.ID)
		replyWithKeyboard(bot, msg, "Search flow started. Please enter PIN:", flowKeyboard(false))
		return
	case "help":
		reply(bot, msg, helpText())
		return
	case "cancel":
		flows.clear(msg.Chat.ID, msg.From.ID)
		reply(bot, msg, "Canceled.")
		return
	}

	f, ok := flows.get(msg.Chat.ID, msg.From.ID)
	if !ok {
		return
	}
	switch f.Kind {
	case flowAdd:
		handleAddFlowStep(db, bot, msg, f)
	case flowSearch:
		handleSearchFlowStep(db, bot, msg, f)
	}
}

func handleCallback(db *sql.DB, bot *tgbotapi.BotAPI, cb *tgbotapi.CallbackQuery) {
	data := cb.Data
	if strings.HasPrefix(data, "menu:add") {
		startAddFlow(bot, cb.Message.Chat.ID, cb.From.ID)
		answerCallback(bot, cb, "Add flow started")
		replyWithKeyboard(bot, cb.Message, "Please enter PIN:", flowKeyboard(false))
		return
	}
	if strings.HasPrefix(data, "menu:search") {
		startSearchFlow(bot, cb.Message.Chat.ID, cb.From.ID)
		answerCallback(bot, cb, "Search flow started")
		replyWithKeyboard(bot, cb.Message, "Please enter PIN:", flowKeyboard(false))
		return
	}
	if strings.HasPrefix(data, "menu:help") {
		answerCallback(bot, cb, "Help")
		reply(bot, cb.Message, helpText())
		return
	}
	if strings.HasPrefix(data, "star:") {
		idStr := strings.TrimPrefix(data, "star:")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			answerCallback(bot, cb, "Invalid id")
			return
		}
		added, err := starCoupon(db, id, cb.From.ID, cb.From.UserName)
		if err != nil {
			answerCallback(bot, cb, "Failed to star")
			return
		}
		if !added {
			answerCallback(bot, cb, "Already starred")
			return
		}
		answerCallback(bot, cb, "Star added")
		return
	}
	if strings.HasPrefix(data, "unstar:") {
		idStr := strings.TrimPrefix(data, "unstar:")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			answerCallback(bot, cb, "Invalid id")
			return
		}
		removed, err := unstarCoupon(db, id, cb.From.ID)
		if err != nil {
			answerCallback(bot, cb, "Failed to unstar")
			return
		}
		if !removed {
			answerCallback(bot, cb, "Not starred")
			return
		}
		answerCallback(bot, cb, "Star removed")
		return
	}
}

func startAddFlow(bot *tgbotapi.BotAPI, chatID, userID int64) {
	flows.set(chatID, userID, &flowState{Kind: flowAdd, Step: 0, Data: map[string]string{}})
}

func startSearchFlow(bot *tgbotapi.BotAPI, chatID, userID int64) {
	flows.set(chatID, userID, &flowState{Kind: flowSearch, Step: 0, Data: map[string]string{}})
}

func handleAddFlowStep(db *sql.DB, bot *tgbotapi.BotAPI, msg *tgbotapi.Message, f *flowState) {
	text := strings.TrimSpace(msg.Text)
	if text == "" {
		replyWithKeyboard(bot, msg, "Please enter a value.", flowKeyboard(true))
		return
	}
	if isSkip(text) {
		if f.Step == 4 || f.Step == 5 {
			handleAddFlowSkip(db, bot, msg, f)
		} else {
			replyWithKeyboard(bot, msg, "Skip not available for this step.", flowKeyboard(true))
		}
		return
	}
	switch f.Step {
	case 0:
		if err := verifyGlobalPin(db, text); err != nil {
			if err == errPinNotSet {
				replyWithKeyboard(bot, msg, "No PIN set. Use /setpin 2512 first.", flowKeyboard(false))
				return
			}
			replyWithKeyboard(bot, msg, "Invalid PIN.", flowKeyboard(false))
			return
		}
		f.Step = 1
		flows.set(msg.Chat.ID, msg.From.ID, f)
		replyWithKeyboard(bot, msg, "PIN OK. Enter coupon value:", flowKeyboard(true))
	case 1:
		f.Data["value"] = text
		f.Step = 2
		flows.set(msg.Chat.ID, msg.From.ID, f)
		replyWithKeyboard(bot, msg, "Enter platform (website name):", flowKeyboard(true))
	case 2:
		f.Data["platform"] = text
		f.Step = 3
		flows.set(msg.Chat.ID, msg.From.ID, f)
		replyWithKeyboard(bot, msg, "Enter year (YYYY):", flowKeyboard(true))
	case 3:
		year, err := strconv.Atoi(text)
		if err != nil || year <= 0 {
			replyWithKeyboard(bot, msg, "Year must be a valid number.", flowKeyboard(true))
			return
		}
		f.Data["year"] = text
		f.Step = 4
		flows.set(msg.Chat.ID, msg.From.ID, f)
		replyWithKeyboard(bot, msg, "Enter month (1-12) or skip for yearly coupon:", flowKeyboard(true))
	case 4:
		month, err := strconv.Atoi(text)
		if err != nil || month < 1 || month > 12 {
			replyWithKeyboard(bot, msg, "Month must be 1-12.", flowKeyboard(true))
			return
		}
		f.Data["month"] = text
		f.Step = 5
		flows.set(msg.Chat.ID, msg.From.ID, f)
		replyWithKeyboard(bot, msg, "Enter tags comma-separated or skip:", flowKeyboard(true))
	case 5:
		f.Data["tags"] = text
		createFromFlow(db, bot, msg, f)
	}
}

func handleAddFlowSkip(db *sql.DB, bot *tgbotapi.BotAPI, msg *tgbotapi.Message, f *flowState) {
	switch f.Step {
	case 4:
		f.Step = 5
		flows.set(msg.Chat.ID, msg.From.ID, f)
		replyWithKeyboard(bot, msg, "Enter tags comma-separated or skip:", flowKeyboard(true))
	case 5:
		createFromFlow(db, bot, msg, f)
	}
}

func createFromFlow(db *sql.DB, bot *tgbotapi.BotAPI, msg *tgbotapi.Message, f *flowState) {
	monthStr := f.Data["month"]
	var month *int
	if monthStr != "" {
		m, err := strconv.Atoi(monthStr)
		if err == nil && m >= 1 && m <= 12 {
			month = &m
		}
	}
	year, _ := strconv.Atoi(f.Data["year"])
	id, err := createCoupon(db, Coupon{
		Value:    f.Data["value"],
		Platform: f.Data["platform"],
		Year:     year,
		Month:    month,
		UserID:   msg.From.ID,
		Username: msg.From.UserName,
		Tags:     parseTags(f.Data["tags"]),
	})
	flows.clear(msg.Chat.ID, msg.From.ID)
	if err != nil {
		reply(bot, msg, "Failed to add coupon: "+err.Error())
		return
	}
	reply(bot, msg, fmt.Sprintf("Coupon added with ID %d", id))
}

func handleSearchFlowStep(db *sql.DB, bot *tgbotapi.BotAPI, msg *tgbotapi.Message, f *flowState) {
	text := strings.TrimSpace(msg.Text)
	if text == "" {
		replyWithKeyboard(bot, msg, "Please enter a value.", flowKeyboard(true))
		return
	}
	if isSkip(text) {
		if f.Step == 2 || f.Step == 3 {
			handleSearchFlowSkip(db, bot, msg, f)
		} else {
			replyWithKeyboard(bot, msg, "Skip not available for this step.", flowKeyboard(true))
		}
		return
	}
	switch f.Step {
	case 0:
		if err := verifyGlobalPin(db, text); err != nil {
			if err == errPinNotSet {
				replyWithKeyboard(bot, msg, "No PIN set. Use /setpin 2512 first.", flowKeyboard(false))
				return
			}
			replyWithKeyboard(bot, msg, "Invalid PIN.", flowKeyboard(false))
			return
		}
		f.Step = 1
		flows.set(msg.Chat.ID, msg.From.ID, f)
		replyWithKeyboard(bot, msg, "PIN OK. Enter year (YYYY):", flowKeyboard(true))
	case 1:
		year, err := strconv.Atoi(text)
		if err != nil || year <= 0 {
			replyWithKeyboard(bot, msg, "Year must be a valid number.", flowKeyboard(true))
			return
		}
		f.Data["year"] = text
		f.Step = 2
		flows.set(msg.Chat.ID, msg.From.ID, f)
		replyWithKeyboard(bot, msg, "Enter platform (partial name) or skip:", flowKeyboard(true))
	case 2:
		f.Data["platform"] = text
		f.Step = 3
		flows.set(msg.Chat.ID, msg.From.ID, f)
		replyWithKeyboard(bot, msg, "Enter month (1-12) or skip:", flowKeyboard(true))
	case 3:
		month, err := strconv.Atoi(text)
		if err != nil || month < 1 || month > 12 {
			replyWithKeyboard(bot, msg, "Month must be 1-12.", flowKeyboard(true))
			return
		}
		f.Data["month"] = text
		runSearchFlow(db, bot, msg, f)
	}
}

func handleSearchFlowSkip(db *sql.DB, bot *tgbotapi.BotAPI, msg *tgbotapi.Message, f *flowState) {
	switch f.Step {
	case 2:
		f.Step = 3
		flows.set(msg.Chat.ID, msg.From.ID, f)
		replyWithKeyboard(bot, msg, "Enter month (1-12) or skip:", flowKeyboard(true))
	case 3:
		runSearchFlow(db, bot, msg, f)
	}
}

func runSearchFlow(db *sql.DB, bot *tgbotapi.BotAPI, msg *tgbotapi.Message, f *flowState) {
	year, _ := strconv.Atoi(f.Data["year"])
	var platformPtr *string
	if v := strings.TrimSpace(f.Data["platform"]); v != "" {
		platformPtr = &v
	}
	var month *int
	if v := strings.TrimSpace(f.Data["month"]); v != "" {
		m, err := strconv.Atoi(v)
		if err == nil && m >= 1 && m <= 12 {
			month = &m
		}
	}
	flows.clear(msg.Chat.ID, msg.From.ID)
	coupons, err := searchCoupons(db, platformPtr, year, month, maxSearchResults)
	if err != nil {
		reply(bot, msg, "Search failed: "+err.Error())
		return
	}
	if len(coupons) == 0 {
		reply(bot, msg, "No coupons found")
		return
	}
	sendSearchResults(bot, msg, coupons)
}

func sendSearchResults(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, coupons []Coupon) {
	var b strings.Builder
	b.WriteString("Results:\n")
	for _, c := range coupons {
		b.WriteString(fmt.Sprintf("#%d | %s | %s | %d-%s | likes:%d | by @%s\n", c.ID, c.Value, c.Platform, c.Year, formatMonth(c.Month), c.Likes, safeUsername(c.Username)))
		if len(c.Tags) > 0 {
			b.WriteString("tags: " + strings.Join(c.Tags, ",") + "\n")
		}
		b.WriteString("\n")
	}
	m := tgbotapi.NewMessage(msg.Chat.ID, b.String())
	m.ReplyToMessageID = msg.MessageID
	m.ReplyMarkup = resultButtons(coupons)
	_, _ = bot.Send(m)
}

func sendMainMenu(bot *tgbotapi.BotAPI, chatID int64) {
	m := tgbotapi.NewMessage(chatID, "Choose an action:")
	m.ReplyMarkup = mainMenuKeyboard()
	_, _ = bot.Send(m)
}

func mainMenuKeyboard() tgbotapi.ReplyKeyboardMarkup {
	return tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Add Coupon"),
			tgbotapi.NewKeyboardButton("Search"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Help"),
			tgbotapi.NewKeyboardButton("Cancel"),
		),
	)
}

func resultButtons(coupons []Coupon) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, c := range coupons {
		row := tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("⭐ %d", c.ID), fmt.Sprintf("star:%d", c.ID)),
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("Unstar %d", c.ID), fmt.Sprintf("unstar:%d", c.ID)),
		)
		rows = append(rows, row)
	}
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func answerCallback(bot *tgbotapi.BotAPI, cb *tgbotapi.CallbackQuery, text string) {
	resp := tgbotapi.NewCallback(cb.ID, text)
	_, _ = bot.Request(resp)
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
	if msg.Chat.IsPrivate() {
		kb := mainMenuKeyboard()
		kb.ResizeKeyboard = true
		kb.OneTimeKeyboard = false
		m.ReplyMarkup = kb
	}
	_, _ = bot.Send(m)
}

func handlePin(db *sql.DB, bot *tgbotapi.BotAPI, msg *tgbotapi.Message, args string) {
	pin := strings.TrimSpace(args)
	if !isValidPin(pin) {
		reply(bot, msg, "PIN must be exactly 4 digits. Example: /pin 1234")
		return
	}
	if err := verifyGlobalPin(db, pin); err != nil {
		if err == errPinNotSet {
			reply(bot, msg, "No PIN set in configurations.")
			return
		}
		reply(bot, msg, "Invalid PIN.")
		return
	}
	reply(bot, msg, "PIN verified.")
}

func isValidPin(pin string) bool {
	if len(pin) != 4 {
		return false
	}
	for i := 0; i < len(pin); i++ {
		if pin[i] < '0' || pin[i] > '9' {
			return false
		}
	}
	return true
}

func verifyPinFromKV(db *sql.DB, bot *tgbotapi.BotAPI, msg *tgbotapi.Message, kv map[string]string) bool {
	pin := strings.TrimSpace(kv["pin"])
	if !isValidPin(pin) {
		reply(bot, msg, "PIN required. Example: /add pin=2512 value=... platform=... year=YYYY")
		return false
	}
	if err := verifyGlobalPin(db, pin); err != nil {
		if err == errPinNotSet {
			reply(bot, msg, "No PIN set. Use /setpin 2512 first.")
			return false
		}
		reply(bot, msg, "Invalid PIN.")
		return false
	}
	return true
}

func replyWithKeyboard(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, text string, kb tgbotapi.ReplyKeyboardMarkup) {
	m := tgbotapi.NewMessage(msg.Chat.ID, text)
	m.ReplyToMessageID = msg.MessageID
	if msg.Chat.IsPrivate() {
		kb.ResizeKeyboard = true
		kb.OneTimeKeyboard = false
		m.ReplyMarkup = kb
	}
	_, _ = bot.Send(m)
}

func flowKeyboard(showSkip bool) tgbotapi.ReplyKeyboardMarkup {
	if showSkip {
		return tgbotapi.NewReplyKeyboard(
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("Skip"),
				tgbotapi.NewKeyboardButton("Cancel"),
			),
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("Add Coupon"),
				tgbotapi.NewKeyboardButton("Search"),
			),
		)
	}
	return mainMenuKeyboard()
}

func isSkip(text string) bool {
	return strings.EqualFold(strings.TrimSpace(text), "skip")
}

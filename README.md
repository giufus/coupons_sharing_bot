# Coupon Sharing Telegram Bot (Go + SQLite)

## Setup

1. Create a bot with BotFather and get the token.
2. Set the global PIN in the DB (stores a bcrypt hash):

```bash
go run . pin-set --db coupons.db --pin 1234
```

3. Set env vars and run:

```bash
export BOT_TOKEN="..."
export DB_PATH="coupons.db" # optional

go run .
```

## Commands

```
/pin 1234
/add pin=1234 value=... platform=... year=YYYY month=MM tags=tag1,tag2
/update id=ID [value=...] [platform=...] [year=YYYY] [month=MM|month=] [tags=tag1,tag2]
/delete id=ID
/search pin=1234 year=YYYY [platform=...] [month=MM]
/star id=ID
/unstar id=ID
/cancel
```

Notes:
- Use quotes for values with spaces, e.g. `value="SUMMER 20"`.
- `month=` clears the month (for yearly coupons).
- `platform` search is case-insensitive and supports partial match.
- The on-screen reply keyboard shows Add/Search/Help/Cancel while chatting in private.
- You must provide the global PIN before add/search (flows ask for PIN first).
- The PIN is global (shared), stored in `configurations` as a hash. You must insert it manually.

## Data

SQLite tables:
- `coupons`
- `coupon_stars`

Indexes are created on `platform`, `year`, `month`, and `(platform, year, month)` to speed up searches.

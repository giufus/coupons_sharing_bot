# Coupon Sharing Telegram Bot (Go + SQLite)

## Setup

1. Create a bot with BotFather and get the token.
2. Set env vars and run:

```bash
export BOT_TOKEN="..."
export DB_PATH="coupons.db" # optional

go run .
```

## Commands

```
/add value=... platform=... year=YYYY month=MM tags=tag1,tag2
/update id=ID [value=...] [platform=...] [year=YYYY] [month=MM|month=] [tags=tag1,tag2]
/delete id=ID
/search year=YYYY [platform=...] [month=MM]
/star id=ID
/unstar id=ID
```

Notes:
- Use quotes for values with spaces, e.g. `value="SUMMER 20"`.
- `month=` clears the month (for yearly coupons).
- `platform` search is case-insensitive and supports partial match.

## Data

SQLite tables:
- `coupons`
- `coupon_stars`

Indexes are created on `platform`, `year`, `month`, and `(platform, year, month)` to speed up searches.

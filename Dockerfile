# syntax=docker/dockerfile:1

FROM golang:1.22-bookworm AS builder

ARG REPO_URL=https://github.com/giufus/coupons_sharing_bot.git
ARG REPO_REF=develop

RUN apt-get update && apt-get install -y --no-install-recommends \
  git ca-certificates build-essential \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /src
RUN git clone --depth 1 --branch "${REPO_REF}" "${REPO_URL}" .

# Cache deps.
RUN go mod download

# Build the first discovered main package.
RUN set -eux; \
  mainpkg="$(go list -f '{{if eq .Name "main"}}{{.ImportPath}}{{end}}' ./... | sed '/^$/d' | head -n1)"; \
  if [ -z "$mainpkg" ]; then echo "No main package found"; exit 1; fi; \
  CGO_ENABLED=1 go build -trimpath -ldflags='-s -w' -o /out/bot "$mainpkg"

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /out/bot /app/bot

RUN mkdir -p /data

ENV DB_PATH=/data/coupons.db

ENTRYPOINT ["/app/bot"]

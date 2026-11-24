# 9itchi-chat-api

Admin-user live support chat. PostgreSQL is the source of truth. Realtime delivery goes through the `core.Realtime` interface: Ably (REST publish to channel = conversation id, event `message`) when `ABLY_API_KEY` is set, otherwise a no-op. Publishing happens after the transaction commits and a failure never fails the stored message. Without the key `GET /chat/ably-token` returns `503` `{"error":"realtime unavailable"}`. The service publishes under its own Ably client id (`core.PublisherClientID`), because the Ably app requires identified clients; the tokens it signs are for the caller only.

Port `8088`, health `/health`. Callers and roles come from the JWT (`user_id`, `role`). A user who is not a participant of a conversation gets `404`.

## Endpoints

| Method | Path | Access | Handler | Notes |
|---|---|---|---|---|
| GET | `/conversations` | user | ListConversations | Own conversations, `page`/`page_size` (max 100), last message preview + caller's unread count, `updated_at` desc |
| POST | `/admin/conversations` | admin | StartConversation | Body `{"user_id"}`. Idempotent per admin+user pair, returns the existing conversation |
| PUT | `/conversations/:id/read` | user | MarkConversationRead | Resets the caller's unread count |
| GET | `/user/global-unread-count` | user | GlobalUnreadCount | Sum of the caller's unread counts |
| GET | `/chat/ably-token` | user | AblyToken | Ably TokenRequest (30 min TTL, `clientId` = caller). Capability: subscribe/presence/history (no publish: only SendMessage publishes) on the caller's conversation channels (100 most recent), or subscribe on a placeholder channel when none. 503 without `ABLY_API_KEY` |
| GET | `/conversations/:id/messages` | user | ListMessages | Newest first, `page`/`page_size` (max 100), optional `before` (RFC3339) cursor on `created_at` |
| POST | `/conversations/:id/messages` | user | SendMessage | Body `{"content"}` trimmed, 1..4000 chars. One transaction: insert, bump `updated_at`, increment the other participant's unread count; then publish on channel = conversation id |

## Run

```
cp .env.example .env
go run main.go
```

## Test

```
go test ./...
go test -tags integration -count=1 ./repositories/...   # embedded Postgres, ~1 min
```

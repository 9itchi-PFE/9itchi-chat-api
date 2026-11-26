# 9itchi Chat API
Student-to-team support chat. Postgres is the source of truth, Ably the live layer. Port 8088, DB `chat`. Publishes one event over RabbitMQ (`support.replied`).

## Conventions (shared by every 9itchi Go service)
- `handlers/` thin (validate, authorize, call service, JSON) -> `services/` all logic, returns `(T, *models.ApiError)` -> `repositories/` only Postgres, plain errors (`ErrNotFound`, `ErrConflict`). Error body `{"error": "..."}`.
- DB calls use `repositories.QueryTimeout`; multi-table writes `Store.WithTx`.
- Schema: add `migrations/NNNN_name.sql` (goose, embedded, runs at startup). Never edit a merged migration.
- Auth: `Authorization: Bearer` AES-GCM-wrapped HS256 JWT (claims `id`, `email`, `role`, `typ`, `exp`; access only). `/admin/*` = `RequireRole("admin")`. `/internal/*` = `x-api-key` (`INTERNAL_API_KEY`), blocked by nginx.
- Env only, validated in `utils/config.go`. Common: `PORT`, `ENVIRONMENT`, `DATABASE_URL`, `ITCHI_SECRET_KEY`, `ITCHI_ENCRYPTION_KEY`, `INTERNAL_API_KEY`, `INTERNAL_API_KEY_REQUIRED`. Never log tokens, passwords, PII.
- Tests: `{resource}_test.go`, testify, func-field fakes (no testify/mock), success + error paths. Repo tests: `//go:build integration`, embedded Postgres.
- Check: `gofmt -l .; go build ./... && go vet ./... && go test ./...`; `go test -tags integration -count=1 ./repositories/...`; `golangci-lint run`.

## Routes
- User (`AuthMiddleware` + `SupportSeat`: an admin acts as the team seat): `GET|POST /conversations` (POST 403 for admin), `PUT /conversations/:id/read`, `GET|POST /conversations/:id/messages`, `GET /user/global-unread-count`, `GET /chat/ably-token` (503 without Ably).
- Admin: `POST /admin/conversations` (`user_id`), `PUT /admin/conversations/:id/resolved` (`{resolved}`).
- Internal: `DELETE /internal/users/:id`.

## Data, providers
- Tables: `conversations` (`resolved_at`), `conversation_participants` (`unread_count` per seat, `reply_emailed`), `messages`. Latest `0004_reply_email.sql`.
- `core.Realtime`: Ably if `ABLY_API_KEY` else no-op. Server publishes event `message` on channel = conversation id, as `core.PublisherClientID`.
- Events: exchange `9itchi.events`, names in `core/events.go`; empty `RABBITMQ_URL` = `LogPublisher`. `services.ReplyNotifier` (started by `app.Run`) sweeps every minute: a student seat with unread team messages, not yet emailed in this unread streak, in a conversation quiet for 2 min, is claimed (`reply_emailed`, `FOR UPDATE SKIP LOCKED`) and `support.replied {user_id, conversation_id, unread}` is published (never the text); publish failure gives the seat back. `MarkRead` clears the flag. notification-api sends the email.
- Env: `ABLY_API_KEY`, `ACCOUNT_API_URL`, `NOTIFICATION_API_URL`, `RABBITMQ_URL`.

## Domain rules
- Team inbox: one conversation per student, held by team seat `00000000-0000-4000-8000-000000000000` (`0002_team_inbox.sql`); the writing admin stays the sender.
- Message 1-4000 chars (DB CHECK + service). Non-participant = 404. Lists `page`/`page_size` (20, max 100); messages newest first, `before` cursor.
- Resolved conversations stay listed; a new message reopens them.
- Ably publish after commit on a detached context; failure only logs. Clients never publish: token = subscribe/presence/history on the 100 latest conversation channels, TTL 30 min.

## Gotchas
- `pkg/account` and `pkg/notification` clients are built but unused.

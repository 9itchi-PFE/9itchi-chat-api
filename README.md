# 9itchi-chat-api

Admin-user live support chat. PostgreSQL is the source of truth. Realtime delivery goes through the `core.Realtime` interface: Ably (REST publish to channel = conversation id, event `message`) when `ABLY_API_KEY` is set, otherwise a no-op. Publishing happens after the transaction commits and a failure never fails the stored message. Without the key `GET /chat/ably-token` returns `503` `{"error":"realtime unavailable"}`. The service publishes under its own Ably client id (`core.PublisherClientID`), because the Ably app requires identified clients; the tokens it signs are for the caller only.

Port `8088`, health `/health`. Callers and roles come from the JWT (`user_id`, `role`). A user who is not a participant of a conversation gets `404`.

## Endpoints

| Method | Path | Access | Handler | Notes |
|---|---|---|---|---|
| GET | `/conversations` | user | ListConversations | The conversations of the caller's seat (every student for an admin), `page`/`page_size` (max 100), last message preview + caller's unread count, `updated_at` desc |
| POST | `/conversations` | user | OpenSupport | The student's conversation with the team, created on first use. 403 for an admin |
| POST | `/admin/conversations` | admin | StartConversation | Body `{"user_id"}`. The team's conversation with that student, created on first use |
| PUT | `/admin/conversations/:id/resolved` | admin | SetResolved | Body `{"resolved": true}`. A resolved conversation stays in the inbox with `resolved_at`; any new message opens it again |
| PUT | `/conversations/:id/read` | user | MarkConversationRead | Resets the caller's unread count |
| GET | `/user/global-unread-count` | user | GlobalUnreadCount | Sum of the caller's unread counts |
| GET | `/chat/ably-token` | user | AblyToken | Ably TokenRequest (30 min TTL, `clientId` = caller). Capability: subscribe/presence/history (no publish: only SendMessage publishes) on the caller's conversation channels (100 most recent), or subscribe on a placeholder channel when none. 503 without `ABLY_API_KEY` |
| GET | `/conversations/:id/messages` | user | ListMessages | Newest first, `page`/`page_size` (max 100), optional `before` (RFC3339) cursor on `created_at` |
| POST | `/conversations/:id/messages` | user | SendMessage | Body `{"content"}` trimmed, 1..4000 chars. One transaction: insert, bump `updated_at`, increment the other participant's unread count; then publish on channel = conversation id |

## Team inbox (sprint 7)

Support is a team inbox. A student has one conversation, with the team as a whole; any admin reads and answers it.

- The team has one seat in every conversation: the fixed id `00000000-0000-4000-8000-000000000000` (`core.TeamID`), returned as `admin_id`. It has one unread count, shared by the admins.
- A caller acts from a seat (`SupportSeat` middleware): the team for an admin, themselves for a student. Lists, reading, unread counts, the Ably token and the right to write go by seat.
- A message keeps who wrote it: `sender_id` is the admin, not the team. For the student, any `sender_id` other than their own is the team.
- A student opens their conversation with `POST /conversations`; the team opens one with a student with `POST /admin/conversations`. Both return the same conversation.
- Migration `0002_team_inbox.sql` moves existing conversations to the team: per student the most recent one is kept and the messages of the others move into it. It cannot be undone.

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

## Account deletion

`DELETE /internal/users/:id` (`x-api-key`) is called by account-api before it deletes an account. Here it concerns the support conversations of the student, with their messages. It runs in one transaction, answers `204`, and can be called again without harm (account-api retries until every service has answered). Nothing about the user is logged.

## Email when the team replies

The chat has no email of its own: `services.ReplyNotifier` publishes `support.replied` (`{user_id, conversation_id, unread}`, never the text) on `9itchi.events`, and notification-api sends the email (fr/en/ar, button to `/support`).

- Once per unread streak: the student's seat is flagged `reply_emailed` when the event leaves, and `PUT /conversations/:id/read` clears the flag.
- Only after the conversation has been quiet for 2 minutes, so a student reading the chat live gets nothing, and several messages of the team make one email.
- The sweep runs every minute from the database (`FOR UPDATE SKIP LOCKED`): a restart loses nothing and two instances never send twice. If the event cannot be published, the seat is given back and tried at the next sweep.
- Unread messages from before migration `0004` count as already told.

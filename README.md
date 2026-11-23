# 9itchi-chat-api

Admin-user live support chat. MongoDB is source of truth, Ably delivers, presence and unread counts.

Port `8088`, health `/health`. Skeleton only: every endpoint returns `501 not implemented` until built.

## Endpoints

| Method | Path | Access | Handler |
|---|---|---|---|
| GET | `/conversations` | user | ListConversations |
| POST | `/admin/conversations` | admin | StartConversation |
| PUT | `/conversations/:id/read` | user | MarkConversationRead |
| GET | `/user/global-unread-count` | user | GlobalUnreadCount |
| GET | `/chat/ably-token` | user | AblyToken |
| GET | `/conversations/:id/messages` | user | ListMessages |
| POST | `/conversations/:id/messages` | user | SendMessage |

## Run

```
cp .env.example .env
go run main.go
```

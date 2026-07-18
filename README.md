# Gmail Notification

Production-oriented **Flutter (Android)** + **Go** stack that watches authorized Gmail inboxes via OAuth 2.0 and delivers real-time alerts through **Firebase Cloud Messaging**. A web UI is also served by the backend. Gmail passwords are never requested or stored — only OAuth tokens (encrypted at rest).

## Architecture

```
mobile/     Flutter Android app (Material 3, Riverpod, secure storage, FCM)
backend/    Go API + web UI (clean architecture, JWT, Gmail watch/history, FCM, Postgres)
```

**Flow**

1. User signs in with email/password **or** Gmail QR OAuth (scan-to-login).
2. User connects one or more Gmail accounts via Google OAuth (offline access + refresh token).
3. Backend encrypts tokens (AES-256-GCM), starts `users.watch` on a Pub/Sub topic, and stores `historyId`.
4. Gmail push (or history poll fallback) → backend fetches new messages → applies notification filters → FCM.
5. Web/mobile clients show notification history and settings.

## Prerequisites

- Docker / Docker Compose (recommended), or Go 1.22+
- Flutter 3.22+ (for the Android app)
- Google Cloud project with Gmail API (+ Pub/Sub for push)
- Firebase project (Android + FCM service account) for push to devices

## Docker (recommended)

Images:

| Image | Tag | Description |
|-------|-----|-------------|
| `gmail-notification-api` | `latest`, `1.1.0` | API + web UI on port **8088** |
| `postgres` | `16-alpine` | Database (host port **5438**) |

### Build and run

```bash
cd backend
cp .env.example .env
# Edit .env: JWT_SECRET, TOKEN_ENCRYPTION_KEY, GOOGLE_CLIENT_ID/SECRET
# Optional: credentials/firebase-service-account.json

docker compose build
docker compose up -d
```

Or in one step:

```bash
docker compose up --build -d
```

Open the web app: **http://localhost:8088**  
Health check: **http://localhost:8088/healthz**

Useful commands:

```bash
docker compose logs -f api
docker compose ps
docker compose down          # stop
docker compose down -v       # stop and wipe Postgres volume (re-runs migrations)
```

### Build the image alone

```bash
cd backend
docker build -t gmail-notification-api:latest -t gmail-notification-api:1.1.0 .
```

### Ports

| Service | Host | Container |
|---------|------|-----------|
| Web UI + API | `8088` | `8088` |
| PostgreSQL | `5438` | `5432` |

Host Postgres uses **5438** so it does not collide with other local databases on `5432`.

### Migrations

On **first** Postgres start, scripts in `migrations/` are applied automatically (`001_init.sql`, `002_qr_login.sql`).

If the volume already exists and you add a new migration:

```bash
# PowerShell
Get-Content .\migrations\002_qr_login.sql | docker exec -i backend-postgres-1 psql -U gmail_user -d gmail_notifications
```

Or reset: `docker compose down -v && docker compose up --build -d`.

## Web UI features

- Login / register (JWT)
- **Sign in with Gmail QR code** (popup + scan / open link)
- Gmail account link / unlink
- Notification history and filters
- Dark mode

QR notes: scanning a `localhost` QR from a phone will not reach your PC. Use **Open link on this device**, or set `APP_BASE_URL` to your LAN IP (e.g. `http://192.168.1.10:8088`) and register that redirect URI in Google Cloud.

## Backend without Docker

```bash
cd backend
cp .env.example .env
# Start Postgres and apply migrations/*.sql
go run ./cmd/server
```

Tests:

```bash
cd backend
go test ./...
```

## Key REST endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/v1/auth/register` | — | Create app user |
| POST | `/api/v1/auth/login` | — | JWT login |
| POST | `/api/v1/auth/refresh` | — | Rotate refresh token |
| POST | `/api/v1/auth/qr/session` | — | Create QR login session |
| GET | `/api/v1/auth/qr/{id}/start` | — | Phone: start Google OAuth |
| GET | `/api/v1/auth/qr/{id}/status` | — | Poll QR login status |
| GET | `/api/v1/auth/me` | JWT | Current user |
| POST | `/api/v1/gmail/accounts/link` | JWT | Start Google OAuth link |
| GET | `/api/v1/oauth/google/callback` | — | OAuth redirect |
| GET | `/api/v1/gmail/accounts` | JWT | List linked accounts |
| DELETE | `/api/v1/gmail/accounts/{id}` | JWT | Unlink |
| GET | `/api/v1/notifications` | JWT | History |
| GET/PUT | `/api/v1/settings/notifications` | JWT | Filters / quiet hours |
| POST | `/api/v1/devices` | JWT | Register FCM token |
| POST | `/api/v1/webhooks/gmail-pubsub` | Pub/Sub | Gmail push webhook |

## Mobile (Flutter)

```bash
cd mobile
flutter create . --project-name gmail_notification --org com.gmailnotify --platforms=android
# Add android/app/google-services.json from Firebase when enabling FCM
flutter pub get
flutter run --dart-define=API_BASE_URL=http://10.0.2.2:8088/api/v1
```

Use your machine LAN IP instead of `10.0.2.2` on a physical device.

App features: login/register, multi-account Gmail OAuth, notification history, filters, dark mode, FCM.

## Google Cloud setup

1. Enable **Gmail API**.
2. Create OAuth **Web** client; redirect URI: `http://localhost:8088/api/v1/oauth/google/callback` (and LAN/public URL if used).
3. Create Pub/Sub topic; grant `gmail-api-push@system.gserviceaccount.com` publish rights.
4. Create a **push** subscription to `https://YOUR_PUBLIC_HOST/api/v1/webhooks/gmail-pubsub`.
5. Set `GMAIL_PUBSUB_TOPIC=projects/PROJECT/topics/TOPIC`.

For local development, keep `GMAIL_HISTORY_POLL_FALLBACK=true` so history is polled when Pub/Sub is unavailable.

## Configuration

Copy `backend/.env.example` → `backend/.env`. Important keys:

- `APP_PORT` / `APP_BASE_URL` — default `8088` / `http://localhost:8088`
- `JWT_SECRET` — ≥ 32 characters
- `TOKEN_ENCRYPTION_KEY` — 64 hex chars (32-byte AES key)
- `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` / `GOOGLE_REDIRECT_URI`
- `DATABASE_URL` — in Compose this is overridden to the `postgres` service
- `FIREBASE_CREDENTIALS_FILE` — optional; FCM becomes a no-op if missing

## Troubleshooting

**`http://localhost:8088` not responding**

Usually the Postgres container stopped (e.g. after a host reboot or Docker restart), so the API crash-loops with `database connection failed ... lookup postgres ... no such host`. Bring the stack back up:

```bash
cd backend
docker compose up -d
docker compose ps          # both services should be "healthy"
docker compose logs -f api
```

If it persists, do a clean restart:

```bash
docker compose down
docker compose up -d
```

The `api` service uses `depends_on: postgres (service_healthy)` and `restart: unless-stopped`, so it recovers automatically once Postgres is healthy.

**Port already in use** — another process holds `8088` (or `5438`). Stop it, or change the host port mapping in `docker-compose.yml`.

## Security

- OAuth access/refresh tokens encrypted with `TOKEN_ENCRYPTION_KEY` (AES-256-GCM).
- App refresh tokens stored hashed (SHA-256); QR login tokens are one-time consumed.
- JWT access tokens are short-lived; refresh tokens rotate on use.
- Never commit `.env`, `google-services.json`, or Firebase service account JSON.

## Project layout (backend)

```
cmd/server            entrypoint
internal/config       env configuration
internal/domain       entities + repository ports
internal/repository   Postgres adapters
internal/service      auth, QR login, Gmail, sync, FCM, workers
internal/handler      HTTP handlers
internal/middleware   JWT, CORS, rate limit, logging
internal/gmail        Gmail API client (watch/history)
internal/fcm          Firebase Cloud Messaging
web/                  Web UI (login, QR popup, accounts, alerts)
migrations/           SQL schema
Dockerfile            multi-stage production image
docker-compose.yml    api + postgres
```

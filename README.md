# api-hamy

Go REST API (Gin + pgx + sqlc + goose) for sharing photo albums between users.
Metadata lives in Postgres and images live in MinIO, with uploads and downloads
via presigned URLs.

## Stack

- **Go 1.27** + Gin
- **Postgres 17** (pgxpool + sqlc + goose migrations)
- **MinIO** (S3-compatible, `minio-go` client)
- **Docker Compose** for db + minio + migrations + api

## Structure

```
cmd/api/                    startup and manual wiring
cmd/seed/                   idempotent demo data for development
internal/config/            environment variables
internal/domain/            entities, ports (repos, blob store) and errors
internal/service/           use cases: auth, admin, user, album, photo
internal/token/             JWT issuance and validation (HS256)
internal/repository/        Postgres adapter (sqlc + pgx)
internal/storage/minio/     storage adapter (presign, stat, remove)
internal/transport/http/    router, middleware, handlers, DTOs and responses
internal/testutil/          in-memory fakes for tests
migrations/                 SQL migrations (goose)
scripts/                    end-to-end smoke test
docs/                       OpenAPI contract
api.http                    manual walkthrough (REST Client)
```

Dependencies always point toward `domain`:

```
transport/http ──▶ service ──▶ domain ◀── repository/postgres
                                    ◀── storage/minio
```

`domain` does not import Gin, pgx, sqlc or minio.

## Getting started

### Everything in Docker (recommended)

```bash
cp .env.example .env
make up          # db + minio + migrations + api
curl localhost:8080/healthz
curl localhost:8080/readyz   # checks Postgres and MinIO

make seed        # demo accounts and album (known credentials)
make smoke       # full end-to-end flow
```

MinIO console: <http://localhost:9001> (`MINIO_ACCESS_KEY` / `MINIO_SECRET_KEY`).

### Native development

```bash
cp .env.example .env
make infra-up    # Postgres + MinIO
make migrate-up
make run
```

## Environment variables

| Variable | Default | Description |
|---|---|---|
| `APP_ENV` | `development` | Environment |
| `HTTP_PORT` | `8080` | HTTP port |
| `CORS_ALLOWED_ORIGINS` | `*` | Allowed origins (comma-separated list) |
| `JWT_SECRET` | insecure development default | HS256 signing key (at least 32 characters; required with `APP_ENV=production`) |
| `JWT_ISSUER` | `api-hamy` | Issuer (`iss`) required on tokens |
| `JWT_TTL` | `24h` | Access token lifetime |
| `SEED_USER_PASSWORD` | `demo-password` | Password for the demo accounts (`make seed`) |
| `SEED_ADMIN_PASSWORD` | `admin-password` | Password for `admin@example.com` |
| `DB_DSN` | `postgres://hamy:hamy@localhost:5432/hamy?sslmode=disable` | Postgres connection |
| `DB_MAX_CONNS` | `10` | Pool size |
| `DB_TEST_DSN` | empty → uses `DB_DSN` | Database for `make test-integration` |
| `MINIO_ENDPOINT` | `localhost:9000` | Internal endpoint (API → MinIO) |
| `MINIO_PUBLIC_ENDPOINT` | same as internal | Host signed into presigned URLs |
| `MINIO_ACCESS_KEY` / `MINIO_SECRET_KEY` | — | Credentials |
| `MINIO_BUCKET` | `hamy-photos` | Bucket (created at startup if missing) |
| `MINIO_REGION` | `us-east-1` | S3 region |
| `MINIO_USE_SSL` | `false` | TLS toward MinIO |
| `MINIO_PUBLIC_USE_SSL` | same as `MINIO_USE_SSL` | TLS in presigned URLs |
| `MINIO_API_CORS_ALLOW_ORIGIN` | `*` | S3 API CORS (browser uploads) |
| `UPLOAD_URL_TTL` | `15m` | Presigned URL TTL |

> `MINIO_PUBLIC_ENDPOINT` is key in Docker: the API talks to `minio:9000`,
> but signs URLs with `localhost:9000` because they are consumed by a client
> outside the Docker network. If the name does not match, the browser/curl
> cannot resolve it.

## Data model

Migrations live in `migrations/`:

| Table | Role |
|---|---|
| `users` | Users (soft delete, unique email among active users, bcrypt `password_hash`, `is_admin`) |
| `albums` | Album; `owner_id` is the only source of truth for the owner |
| `album_members` | Invitees: `role` (`viewer`/`editor`) and `status` (`invited`/`active`/`revoked`) |
| `photos` | Image metadata; the bytes live in MinIO |
| `album_access` (view) | Effective permissions: owner + active members |

Rules:

- The owner is **not** stored in `album_members` (avoids two sources of truth).
- Inviting = `status='invited'`; accepting = `status='active'`.
- `photos.status`: `pending` until the object is confirmed → `ready`.
- Soft delete (`deleted_at`) on `users`, `albums` and `photos`.
- The `set_updated_at()` trigger maintains `updated_at` on albums, members and photos.

### Permissions

| Action | owner | editor | viewer | no access |
|---|---|---|---|---|
| View album / members / photos | ✅ | ✅ | ✅ | 404 |
| Upload / delete photos | ✅ | ✅ | ❌ 403 | 404 |
| Edit album, cover, invite, change roles | ✅ | ❌ 403 | ❌ 403 | 404 |
| Leave the album | ❌ 422 | ✅ | ✅ | 404 |

Authorization is evaluated in SQL against `album_access`; a user without access
receives 404 (the existence of the album is not disclosed).

## Upload flow

1. `POST /albums/:id/photos/uploads` with `filename`, `content_type`, `size_bytes`.
2. The API creates the `pending` row and returns `upload_url` + `expires_at`.
3. The client uploads the raw binary with a direct `PUT` to MinIO.
4. `POST /albums/:id/photos/:photoId/complete` uses `StatObject` to verify that
   the object exists and that the size matches, then marks the photo `ready`.
5. `GET /photos/:id/download-url` returns a presigned download URL.

Allowed types: `image/jpeg`, `image/png`, `image/webp`, `image/gif`,
`image/heic`, `image/avif`. Maximum: 25 MiB.

## Endpoints

| Method | Path | Description |
|---|---|---|
| GET | `/healthz` | Liveness (no dependencies) |
| GET | `/readyz` | Readiness: Postgres + MinIO (503 on failure) |
| POST | `/api/v1/auth/register` | Create account and get token (public) |
| POST | `/api/v1/auth/login` | Login and token (public) |
| GET | `/api/v1/auth/me` | Authenticated user |
| GET | `/api/v1/admin/users` | List users (admin; `page`, `per_page`, `search`) |
| GET | `/api/v1/admin/users/:id` | Get user (admin) |
| PATCH | `/api/v1/admin/users/:id` | Update profile or `is_admin` (admin) |
| DELETE | `/api/v1/admin/users/:id` | Soft delete user (admin) |
| POST | `/api/v1/albums` | Create album |
| GET | `/api/v1/albums` | Own and shared albums |
| GET | `/api/v1/albums/:id` | Details |
| PATCH | `/api/v1/albums/:id` | Edit title/description (owner) |
| DELETE | `/api/v1/albums/:id` | Soft delete (owner) |
| PUT | `/api/v1/albums/:id/cover` | Set cover photo (owner) |
| DELETE | `/api/v1/albums/:id/cover` | Remove cover photo (owner) |
| POST | `/api/v1/albums/:id/members` | Invite by `user_id` or `email` (owner) |
| GET | `/api/v1/albums/:id/members` | List members |
| PATCH | `/api/v1/albums/:id/members/:userId` | Change role (owner) |
| DELETE | `/api/v1/albums/:id/members/:userId` | Remove member (owner) or leave (self) |
| GET | `/api/v1/invitations` | My pending invitations |
| POST | `/api/v1/invitations/:albumId/accept` | Accept invitation |
| GET | `/api/v1/albums/:id/photos` | List photos |
| POST | `/api/v1/albums/:id/photos/uploads` | Start upload |
| POST | `/api/v1/albums/:id/photos/:photoId/complete` | Confirm upload |
| GET | `/api/v1/photos/:id/download-url` | Download URL |
| DELETE | `/api/v1/photos/:id` | Delete photo (owner/editor or uploader) |

## Testing the API

### Smoke test (recommended)

With the stack running:

```bash
make smoke
```

`scripts/smoke.sh` registers three users, obtains JWT tokens and walks through
the entire flow: 401/403 guards, create album, invite by email, accept, check
permissions, upload a real PNG to the presigned URL, confirm, download and
compare bytes, set the cover, admin module, delete photo, leave the album and
delete the album. It exits with a non-zero code if anything fails. It requires
`curl`, `jq`, `base64` and `cmp`, and must be run from the host (not inside the
container). The admin checks use `SEED_ADMIN_EMAIL`/`SEED_ADMIN_PASSWORD` and are
marked as `skip` if that account does not exist.

### Demo data

```bash
make seed
```

Creates or **resets** the demo accounts and guarantees an administrator:

| Email | Password | Role |
|---|---|---|
| `admin@example.com` | `admin-password` | admin |
| `ana@example.com` | `demo-password` | owner of the demo album |
| `luis@example.com` | `demo-password` | active editor |
| `marta@example.com` | `demo-password` | invited viewer |

Passwords can be changed with `SEED_ADMIN_PASSWORD` and `SEED_USER_PASSWORD`.
The seed also fixes phase 0 users (without `password_hash`). It refuses to run
with `APP_ENV=production` unless `-force` is passed.

### Manual walkthrough

- `api.http`: REST Client collection with all requests in order. Includes
  uploading `scripts/pixel.png` (1×1, 70 bytes) to the presigned URL.
- `docs/openapi.yaml`: OpenAPI 3.0 contract for all endpoints.
- MinIO: the upload is a direct `PUT` to the presigned URL. The bucket has
  open CORS in development (`MINIO_API_CORS_ALLOW_ORIGIN`), needed for testing
  from a browser or Flutter web.

### Tests

```bash
make test              # unit + HTTP tests (with -race)
make coverage          # coverage
make test-integration  # repositories against real Postgres (requires migrations)
```

Service tests cover authentication (register/login, bcrypt hashing,
invalid tokens and deleted users), administration (self-delete and
self-demotion guards), the owner/editor/viewer/no-access permission matrix,
invitations and the photo state machine. Transport tests walk through the
full flow with in-memory fakes, including JWT 401/403 and admin.

### Authentication

`POST /api/v1/auth/register` and `POST /api/v1/auth/login` are public and
return a JWT (HS256, configurable `iss`, expiration `JWT_TTL`). All other
endpoints require `Authorization: Bearer <token>`.

```bash
TOKEN=$(curl -s localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"ana@example.com","password":"demo-password"}' | jq -r .data.access_token)

curl -H "Authorization: Bearer $TOKEN" localhost:8080/api/v1/albums
```

Security decisions:

- Passwords use **bcrypt** (`DefaultCost`, minimum 8 characters and maximum 72
  bytes due to the bcrypt limit). The hash is never returned in JSON.
- The user is **reloaded from Postgres on every request**: deleting an account or
  changing `is_admin` invalidates the token immediately.
- Login with an unknown email performs a padding bcrypt hash so that account
  existence cannot be leaked through timing.
- In production `JWT_SECRET` is required and cannot be the development
  default (minimum 32 characters).
- The admin module (`/api/v1/admin/*`) requires `is_admin=true`. An admin cannot
  remove their own role or delete their own account.

> When sharing an album, **the invitee must have an account**. Inviting by email
> someone who is not registered returns `422` with the message
> `the invited user must have an account`.

## Commands

| Command | What it does |
|---|---|
| `make up` / `make down` | Full Docker stack |
| `make infra-up` | Postgres + MinIO for native development |
| `make run` | Start the API locally |
| `make test` | Unit and HTTP tests with `-race` |
| `make coverage` | Coverage by function (`coverage.out`) |
| `make test-integration` | Repositories against real Postgres (`-tags=integration`) |
| `make seed` | Idempotent demo accounts and album (resets credentials) |
| `make smoke` | End-to-end flow against the running stack |
| `make generate` | Regenerate `sqlcgen` from `queries/*.sql` |
| `make migrate-up` / `make migrate-down` | Apply/revert migrations |
| `make migrate-create name=...` | Create a migration |
| `make ps` / `make logs` | Docker status and logs |

## Notes

- **MinIO image**: `minio/minio` and `quay.io/minio` are no longer public.
  Compose pins `bitnamilegacy/minio:2025.7.23-debian-12-r5` and its digest for
  reproducible deployments (override with `MINIO_IMAGE`). To update, get the
  digest with `docker buildx imagetools inspect bitnamilegacy/minio:<tag>`.
  The code uses standard S3, so in production it can point to your own MinIO
  installation or to S3.
- **Orphans**: if the client never confirms, `pending` rows and objects remain
  in MinIO. The `ListStalePendingPhotos` query is ready for a cleanup job.
- **Upload size**: a presigned PUT does not enforce a size; it is validated on
  confirmation with `StatObject` (if it does not match, the photo does not
  become `ready`).
- **sqlc**: do not edit `internal/repository/postgres/sqlcgen/` by hand.
- **Readiness vs. liveness**: `/healthz` does not touch dependencies (useful for
  the orchestrator); `/readyz` checks Postgres and MinIO with a 2s timeout and
  returns 503 if either fails.
- **CORS**: the API allows the origins from `CORS_ALLOWED_ORIGINS` and accepts
  the `Authorization` header. MinIO needs its own CORS
  (`MINIO_API_CORS_ALLOW_ORIGIN`) because the presigned `PUT` goes straight to
  the bucket from the client.
- **Integration tests**: skipped unless `DB_TEST_DSN` is defined. The data they
  create uses `itest-*` emails and is cleaned up afterwards.
- **Smoke test**: run it from the host, not inside the container, because the
  presigned URLs point to `localhost:9000`.

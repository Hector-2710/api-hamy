# api-hamy

API REST en Go (Gin + pgx + sqlc + goose) para compartir álbumes de fotos entre
usuarios. La metadata vive en Postgres y las imágenes en MinIO, con subida y
descarga mediante URLs prefirmadas.

## Stack

- **Go 1.27** + Gin
- **Postgres 17** (pgxpool + sqlc + migraciones goose)
- **MinIO** (S3 compatible, cliente `minio-go`)
- **Docker Compose** para db + minio + migraciones + api

## Estructura

```
cmd/api/                    arranque y wiring manual
internal/config/            variables de entorno
internal/domain/            entidades, puertos (repos, blob store) y errores
internal/service/           casos de uso: user, album, photo
internal/repository/        adaptador Postgres (sqlc + pgx)
internal/storage/minio/     adaptador de almacenamiento (presign, stat, remove)
internal/transport/http/    router, middleware, handlers, DTOs y respuestas
migrations/                 migraciones SQL (goose)
```

La dependencia apunta siempre hacia `domain`:

```
transport/http ──▶ service ──▶ domain ◀── repository/postgres
                                   ◀── storage/minio
```

`domain` no importa Gin, pgx, sqlc ni minio.

## Puesta en marcha

### Todo en Docker (recomendado)

```bash
cp .env.example .env
make up          # db + minio + migraciones + api
curl localhost:8080/healthz
```

Consola de MinIO: <http://localhost:9001> (`MINIO_ACCESS_KEY` / `MINIO_SECRET_KEY`).

### Desarrollo nativo

```bash
cp .env.example .env
make infra-up    # Postgres + MinIO
make migrate-up
make run
```

## Variables de entorno

| Variable | Default | Descripción |
|---|---|---|
| `APP_ENV` | `development` | Entorno |
| `HTTP_PORT` | `8080` | Puerto HTTP |
| `DB_DSN` | `postgres://hamy:hamy@localhost:5432/hamy?sslmode=disable` | Conexión Postgres |
| `DB_MAX_CONNS` | `10` | Tamaño del pool |
| `MINIO_ENDPOINT` | `localhost:9000` | Endpoint interno (API → MinIO) |
| `MINIO_PUBLIC_ENDPOINT` | igual a interno | Host que se firma en las URLs prefirmadas |
| `MINIO_ACCESS_KEY` / `MINIO_SECRET_KEY` | — | Credenciales |
| `MINIO_BUCKET` | `hamy-photos` | Bucket (se crea al arrancar si falta) |
| `MINIO_REGION` | `us-east-1` | Región S3 |
| `MINIO_USE_SSL` | `false` | TLS hacia MinIO |
| `UPLOAD_URL_TTL` | `15m` | TTL de URLs prefirmadas |

> `MINIO_PUBLIC_ENDPOINT` es clave en Docker: la API habla con `minio:9000`,
> pero firma las URLs con `localhost:9000` porque las consume un cliente fuera
> de la red de Docker. Si el nombre no coincide, el navegador/curl no resuelve.

## Modelo de datos

Migraciones en `migrations/`:

| Tabla | Rol |
|---|---|
| `users` | Usuarios (soft delete, email único entre activos) |
| `albums` | Álbum, `owner_id` es la única fuente de verdad del dueño |
| `album_members` | Invitados: `role` (`viewer`/`editor`) y `status` (`invited`/`active`/`revoked`) |
| `photos` | Metadata de imagen; los bytes están en MinIO |
| `album_access` (vista) | Permisos efectivos: owner + miembros activos |

Reglas:

- El owner **no** se guarda en `album_members` (evita dos fuentes de verdad).
- Invitar = `status='invited'`; aceptar = `status='active'`.
- `photos.status`: `pending` hasta confirmar el objeto → `ready`.
- Borrado lógico (`deleted_at`) en `users`, `albums` y `photos`.
- Trigger `set_updated_at()` mantiene `updated_at` en albums, members y photos.

### Permisos

| Acción | owner | editor | viewer | sin acceso |
|---|---|---|---|---|
| Ver álbum / miembros / fotos | ✅ | ✅ | ✅ | 404 |
| Subir / borrar fotos | ✅ | ✅ | ❌ 403 | 404 |
| Editar álbum, portada, invitar, cambiar roles | ✅ | ❌ 403 | ❌ 403 | 404 |
| Salir del álbum | ❌ 422 | ✅ | ✅ | 404 |

La autorización se evalúa en SQL contra `album_access`; un usuario sin acceso
recibe 404 (no se filtra la existencia del álbum).

## Flujo de subida

1. `POST /albums/:id/photos/uploads` con `filename`, `content_type`, `size_bytes`.
2. La API crea la fila `pending` y devuelve `upload_url` + `expires_at`.
3. El cliente hace `PUT` del binario directo a MinIO.
4. `POST /albums/:id/photos/:photoId/complete` valida con `StatObject` que el
   objeto exista y que el tamaño coincida, y marca la foto `ready`.
5. `GET /photos/:id/download-url` devuelve una URL prefirmada de descarga.

Tipos permitidos: `image/jpeg`, `image/png`, `image/webp`, `image/gif`,
`image/heic`, `image/avif`. Máximo: 25 MiB.

## Endpoints

| Método | Ruta | Descripción |
|---|---|---|
| GET | `/healthz` | Health check |
| POST | `/api/v1/users` | Crear usuario |
| GET | `/api/v1/users` | Listar (`page`, `per_page`, `search`) |
| GET | `/api/v1/users/:id` | Obtener usuario |
| PATCH | `/api/v1/users/:id` | Actualizar usuario |
| DELETE | `/api/v1/users/:id` | Soft delete |
| POST | `/api/v1/albums` | Crear álbum |
| GET | `/api/v1/albums` | Álbumes propios y compartidos |
| GET | `/api/v1/albums/:id` | Detalle |
| PATCH | `/api/v1/albums/:id` | Editar título/descripción (owner) |
| DELETE | `/api/v1/albums/:id` | Soft delete (owner) |
| PUT | `/api/v1/albums/:id/cover` | Fijar portada (owner) |
| DELETE | `/api/v1/albums/:id/cover` | Quitar portada (owner) |
| POST | `/api/v1/albums/:id/members` | Invitar por `user_id` o `email` (owner) |
| GET | `/api/v1/albums/:id/members` | Listar miembros |
| PATCH | `/api/v1/albums/:id/members/:userId` | Cambiar rol (owner) |
| DELETE | `/api/v1/albums/:id/members/:userId` | Expulsar (owner) o salir (uno mismo) |
| GET | `/api/v1/invitations` | Mis invitaciones pendientes |
| POST | `/api/v1/invitations/:albumId/accept` | Aceptar invitación |
| GET | `/api/v1/albums/:id/photos` | Listar fotos |
| POST | `/api/v1/albums/:id/photos/uploads` | Iniciar subida |
| POST | `/api/v1/albums/:id/photos/:photoId/complete` | Confirmar subida |
| GET | `/api/v1/photos/:id/download-url` | URL de descarga |
| DELETE | `/api/v1/photos/:id` | Borrar foto (owner/editor o quien subió) |

### Identidad

La autenticación todavía no está implementada. Los endpoints de álbumes/fotos
leen el header **`X-User-Id`** (UUID) mediante un middleware de desarrollo
(`internal/transport/http/middleware/actor.go`). Reemplazar por JWT antes de
exponer la API.

```bash
curl -H "X-User-Id: $USER_ID" localhost:8080/api/v1/albums
```

## Comandos

| Comando | Qué hace |
|---|---|
| `make up` / `make down` | Stack completo en Docker |
| `make infra-up` | Postgres + MinIO para desarrollo nativo |
| `make run` | Levanta la API en local |
| `make test` | Tests |
| `make generate` | Regenera `sqlcgen` desde `queries/*.sql` |
| `make migrate-up` / `make migrate-down` | Aplica/revierte migraciones |
| `make migrate-create name=...` | Crea una migración |
| `make ps` / `make logs` | Estado y logs de Docker |

## Notas

- **Imagen de MinIO**: `minio/minio` y `quay.io/minio` dejaron de ser públicas.
  El compose fija `bitnamilegacy/minio:2025.7.23-debian-12-r5` y su digest para que
  el despliegue sea reproducible (override con `MINIO_IMAGE`). Para actualizar,
  obtén el digest con `docker buildx imagetools inspect bitnamilegacy/minio:<tag>`.
  El código es S3 estándar, así que en producción se puede apuntar a MinIO propio
  o a S3.
- **Huérfanos**: si el cliente nunca confirma, quedan filas `pending` y objetos
  en MinIO. La query `ListStalePendingPhotos` está lista para un job de limpieza.
- **Tamaño de subida**: un PUT prefirmado no impone tamaño; se valida al
  confirmar con `StatObject` (si no coincide, la foto no pasa a `ready`).
- **sqlc**: no editar `internal/repository/postgres/sqlcgen/` a mano.

-- name: CreateAlbum :one
INSERT INTO albums (owner_id, title, description)
VALUES (sqlc.arg('owner_id'), sqlc.arg('title'), sqlc.narg('description'))
RETURNING *;

-- name: GetAlbumForActor :one
SELECT a.*, aa.role AS access_role
FROM albums a
JOIN album_access aa ON aa.album_id = a.id
WHERE a.id = sqlc.arg('id')
  AND aa.user_id = sqlc.arg('actor_id')
  AND a.deleted_at IS NULL;

-- name: ListAlbumsForActor :many
SELECT a.*, aa.role AS access_role
FROM albums a
JOIN album_access aa ON aa.album_id = a.id
WHERE aa.user_id = sqlc.arg('actor_id')
  AND a.deleted_at IS NULL
  AND (sqlc.arg('search')::text = '' OR a.title ILIKE '%' || sqlc.arg('search')::text || '%')
ORDER BY a.updated_at DESC
LIMIT sqlc.arg('lim') OFFSET sqlc.arg('off');

-- name: CountAlbumsForActor :one
SELECT count(*)
FROM albums a
JOIN album_access aa ON aa.album_id = a.id
WHERE aa.user_id = sqlc.arg('actor_id')
  AND a.deleted_at IS NULL
  AND (sqlc.arg('search')::text = '' OR a.title ILIKE '%' || sqlc.arg('search')::text || '%');

-- name: UpdateAlbum :one
UPDATE albums
SET title = COALESCE(sqlc.narg('title'), title),
    description = CASE
      WHEN sqlc.arg('clear_description')::boolean THEN NULL
      ELSE COALESCE(sqlc.narg('description'), description)
    END
WHERE id = sqlc.arg('id')
  AND owner_id = sqlc.arg('actor_id')
  AND deleted_at IS NULL
RETURNING *;

-- name: SetAlbumCover :one
UPDATE albums a
SET cover_photo_id = sqlc.narg('cover_photo_id')::uuid
WHERE a.id = sqlc.arg('id')
  AND a.owner_id = sqlc.arg('actor_id')
  AND a.deleted_at IS NULL
  AND (
    sqlc.narg('cover_photo_id')::uuid IS NULL
    OR EXISTS (
      SELECT 1
      FROM photos p
      WHERE p.id = sqlc.narg('cover_photo_id')::uuid
        AND p.album_id = a.id
        AND p.status = 'ready'
        AND p.deleted_at IS NULL
    )
  )
RETURNING a.*;

-- name: SoftDeleteAlbum :execrows
UPDATE albums
SET deleted_at = now()
WHERE id = sqlc.arg('id')
  AND owner_id = sqlc.arg('actor_id')
  AND deleted_at IS NULL;

-- name: ClearAlbumCover :execrows
UPDATE albums
SET cover_photo_id = NULL
WHERE cover_photo_id = sqlc.arg('photo_id')::uuid;

-- name: CreatePendingPhoto :one
WITH new_photo AS (SELECT gen_random_uuid() AS id)
INSERT INTO photos (id, album_id, uploaded_by, bucket, object_key, original_filename, content_type, size_bytes)
SELECT n.id,
       sqlc.arg('album_id')::uuid,
       sqlc.arg('uploaded_by')::uuid,
       sqlc.arg('bucket'),
       'albums/' || sqlc.arg('album_id')::text || '/' || n.id::text,
       sqlc.arg('original_filename'),
       sqlc.arg('content_type'),
       sqlc.arg('size_bytes')
FROM new_photo n
WHERE EXISTS (
  SELECT 1
  FROM album_access aa
  WHERE aa.album_id = sqlc.arg('album_id')::uuid
    AND aa.user_id = sqlc.arg('uploaded_by')::uuid
    AND aa.role IN ('owner', 'editor')
)
RETURNING *;

-- name: GetPhotoForActor :one
SELECT p.*
FROM photos p
JOIN album_access aa ON aa.album_id = p.album_id AND aa.user_id = sqlc.arg('actor_id')
WHERE p.id = sqlc.arg('id')
  AND p.deleted_at IS NULL;

-- name: GetPhotoForEditor :one
SELECT p.*
FROM photos p
JOIN album_access aa ON aa.album_id = p.album_id AND aa.user_id = sqlc.arg('actor_id')
WHERE p.id = sqlc.arg('id')
  AND p.deleted_at IS NULL
  AND (aa.role IN ('owner', 'editor') OR p.uploaded_by = sqlc.arg('actor_id'));

-- name: ListPhotosForActor :many
SELECT p.*
FROM photos p
JOIN album_access aa ON aa.album_id = p.album_id AND aa.user_id = sqlc.arg('actor_id')
WHERE p.album_id = sqlc.arg('album_id')
  AND p.deleted_at IS NULL
  AND p.status = 'ready'
  AND (sqlc.arg('search')::text = '' OR p.original_filename ILIKE '%' || sqlc.arg('search')::text || '%')
ORDER BY p.created_at DESC
LIMIT sqlc.arg('lim') OFFSET sqlc.arg('off');

-- name: CountPhotosForActor :one
SELECT count(*)
FROM photos p
JOIN album_access aa ON aa.album_id = p.album_id AND aa.user_id = sqlc.arg('actor_id')
WHERE p.album_id = sqlc.arg('album_id')
  AND p.deleted_at IS NULL
  AND p.status = 'ready'
  AND (sqlc.arg('search')::text = '' OR p.original_filename ILIKE '%' || sqlc.arg('search')::text || '%');

-- name: MarkPhotoReady :one
UPDATE photos
SET status = 'ready',
    width = COALESCE(sqlc.narg('width'), width),
    height = COALESCE(sqlc.narg('height'), height),
    checksum_sha256 = COALESCE(sqlc.narg('checksum'), checksum_sha256)
WHERE id = sqlc.arg('id')
  AND status = 'pending'
  AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeletePhoto :execrows
UPDATE photos
SET deleted_at = now()
WHERE id = sqlc.arg('id')
  AND deleted_at IS NULL;

-- name: ListStalePendingPhotos :many
SELECT *
FROM photos
WHERE status = 'pending'
  AND created_at < sqlc.arg('before')
  AND deleted_at IS NULL
ORDER BY created_at
LIMIT sqlc.arg('lim');

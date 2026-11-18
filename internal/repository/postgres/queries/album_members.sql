-- name: UpsertAlbumMember :one
INSERT INTO album_members (album_id, user_id, role, status, invited_by)
VALUES (sqlc.arg('album_id'), sqlc.arg('user_id'), sqlc.arg('role'), sqlc.arg('status'), sqlc.narg('invited_by'))
ON CONFLICT (album_id, user_id) DO UPDATE
SET role = EXCLUDED.role,
    status = EXCLUDED.status,
    invited_by = EXCLUDED.invited_by
RETURNING *;

-- name: GetAlbumMember :one
SELECT *
FROM album_members
WHERE album_id = sqlc.arg('album_id')
  AND user_id = sqlc.arg('user_id');

-- name: ListAlbumMembers :many
SELECT m.album_id,
       m.user_id,
       m.role,
       m.status,
       m.invited_by,
       m.created_at,
       m.updated_at,
       u.name AS user_name,
       u.email AS user_email
FROM album_members m
JOIN users u ON u.id = m.user_id
JOIN album_access aa ON aa.album_id = m.album_id AND aa.user_id = sqlc.arg('actor_id')
WHERE m.album_id = sqlc.arg('album_id')
  AND m.status <> 'revoked'
ORDER BY m.created_at;

-- name: ActivateAlbumMember :execrows
UPDATE album_members
SET status = 'active'
WHERE album_id = sqlc.arg('album_id')
  AND user_id = sqlc.arg('user_id')
  AND status = 'invited';

-- name: UpdateAlbumMemberRole :execrows
UPDATE album_members
SET role = sqlc.arg('role')
WHERE album_id = sqlc.arg('album_id')
  AND user_id = sqlc.arg('user_id')
  AND status <> 'revoked';

-- name: RevokeAlbumMember :execrows
UPDATE album_members
SET status = 'revoked'
WHERE album_id = sqlc.arg('album_id')
  AND user_id = sqlc.arg('user_id')
  AND status <> 'revoked';

-- name: ListInvitationsForUser :many
SELECT m.album_id,
       m.user_id,
       m.role,
       m.status,
       m.invited_by,
       m.created_at,
       m.updated_at,
       a.title AS album_title,
       inv.name AS invited_by_name,
       inv.email AS invited_by_email
FROM album_members m
JOIN albums a ON a.id = m.album_id AND a.deleted_at IS NULL
LEFT JOIN users inv ON inv.id = m.invited_by
WHERE m.user_id = sqlc.arg('user_id')
  AND m.status = 'invited'
ORDER BY m.created_at DESC;

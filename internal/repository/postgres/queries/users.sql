-- name: CreateUser :one
INSERT INTO users (name, email)
VALUES ($1, $2)
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE lower(email) = lower($1) AND deleted_at IS NULL;

-- name: ListUsers :many
SELECT * FROM users
WHERE deleted_at IS NULL
  AND (@search::text = '' OR name ILIKE '%' || @search || '%' OR email ILIKE '%' || @search || '%')
ORDER BY created_at DESC
LIMIT @lim OFFSET @off;

-- name: CountUsers :one
SELECT count(*) FROM users
WHERE deleted_at IS NULL
  AND (@search::text = '' OR name ILIKE '%' || @search || '%' OR email ILIKE '%' || @search || '%');

-- name: UpdateUser :one
UPDATE users
SET name  = COALESCE(sqlc.narg('name'), name),
    email = COALESCE(sqlc.narg('email'), email),
    updated_at = now()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteUser :execrows
UPDATE users
SET deleted_at = now(), updated_at = now()
WHERE id = $1 AND deleted_at IS NULL;

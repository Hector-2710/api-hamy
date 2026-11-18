-- +goose Up
-- Authentication and administration.
--
-- password_hash is nullable so the migration is safe for databases created
-- during phase 0: those users cannot log in until a password is set (seed or
-- admin). New registrations always set it.
ALTER TABLE users
  ADD COLUMN password_hash text,
  ADD COLUMN is_admin      boolean NOT NULL DEFAULT false;

-- Fast lookup for the admin bootstrap/authorization checks.
CREATE INDEX users_is_admin_idx ON users (is_admin) WHERE is_admin AND deleted_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS users_is_admin_idx;

ALTER TABLE users
  DROP COLUMN is_admin,
  DROP COLUMN password_hash;

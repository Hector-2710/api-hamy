-- +goose Up
CREATE TABLE users (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name       text NOT NULL,
  email      text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);

-- Partial unique index: allows reusing the email of a deleted user.
CREATE UNIQUE INDEX users_email_lower_key ON users (lower(email)) WHERE deleted_at IS NULL;
CREATE INDEX users_deleted_at_idx ON users (deleted_at);

-- +goose Down
DROP TABLE users;

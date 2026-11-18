-- +goose Up
CREATE TABLE albums (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  owner_id       uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  title          text NOT NULL CHECK (length(btrim(title)) BETWEEN 1 AND 150),
  description    text CHECK (description IS NULL OR length(description) <= 2000),
  cover_photo_id uuid, -- FK to photos; added in 00004 (circular FK)
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz
);

CREATE INDEX albums_owner_idx ON albums (owner_id) WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE albums;

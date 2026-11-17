-- +goose Up
-- The owner is NOT stored here: albums.owner_id is the single source of truth.
CREATE TABLE album_members (
  album_id   uuid NOT NULL REFERENCES albums(id) ON DELETE CASCADE,
  user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role       text NOT NULL DEFAULT 'viewer' CHECK (role IN ('viewer', 'editor')),
  status     text NOT NULL DEFAULT 'invited' CHECK (status IN ('invited', 'active', 'revoked')),
  invited_by uuid REFERENCES users(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (album_id, user_id)
);

CREATE INDEX album_members_user_idx ON album_members (user_id, status);

-- Single permissions view: owner plus active members.
-- All authorization queries read from here.
CREATE VIEW album_access AS
  SELECT a.id AS album_id, a.owner_id AS user_id, 'owner'::text AS role, 'active'::text AS status
  FROM albums a
  WHERE a.deleted_at IS NULL
  UNION ALL
  SELECT m.album_id, m.user_id, m.role, m.status
  FROM album_members m
  JOIN albums a ON a.id = m.album_id AND a.deleted_at IS NULL
  WHERE m.status = 'active';

-- +goose Down
DROP VIEW album_access;
DROP TABLE album_members;

-- +goose Up
CREATE TABLE photos (
  id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  album_id          uuid NOT NULL REFERENCES albums(id) ON DELETE CASCADE,
  uploaded_by       uuid REFERENCES users(id) ON DELETE SET NULL,
  bucket            text NOT NULL,
  object_key        text NOT NULL,
  original_filename text NOT NULL,
  content_type      text NOT NULL,
  size_bytes        bigint NOT NULL CHECK (size_bytes > 0),
  width             int CHECK (width IS NULL OR width > 0),
  height            int CHECK (height IS NULL OR height > 0),
  checksum_sha256   text,
  status            text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'ready', 'failed')),
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),
  deleted_at        timestamptz
);

CREATE UNIQUE INDEX photos_object_key_key ON photos (bucket, object_key);
CREATE INDEX photos_album_idx ON photos (album_id, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX photos_pending_idx ON photos (created_at) WHERE status = 'pending';

-- Circular FK: albums.cover_photo_id -> photos.id.
ALTER TABLE albums
  ADD CONSTRAINT albums_cover_photo_id_fkey
  FOREIGN KEY (cover_photo_id) REFERENCES photos(id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE albums DROP CONSTRAINT albums_cover_photo_id_fkey;
DROP TABLE photos;

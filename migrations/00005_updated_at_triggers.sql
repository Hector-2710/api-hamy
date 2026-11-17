-- +goose Up
-- Keeps updated_at consistent even if a query forgets to set it.
-- +goose StatementBegin
CREATE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER albums_set_updated_at
  BEFORE UPDATE ON albums
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER album_members_set_updated_at
  BEFORE UPDATE ON album_members
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER photos_set_updated_at
  BEFORE UPDATE ON photos
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER photos_set_updated_at ON photos;
DROP TRIGGER album_members_set_updated_at ON album_members;
DROP TRIGGER albums_set_updated_at ON albums;
-- +goose StatementBegin
DROP FUNCTION set_updated_at();
-- +goose StatementEnd

-- +goose Up
ALTER TYPE notification_type ADD VALUE 'slack';

-- +goose Down
DELETE FROM notifier WHERE notification_type = 'slack';

ALTER TYPE notification_type RENAME TO notification_type_old;

CREATE TYPE notification_type AS ENUM ('webhook');

ALTER TABLE notifier
  ALTER COLUMN notification_type TYPE notification_type
  USING notification_type::text::notification_type;

DROP TYPE notification_type_old;

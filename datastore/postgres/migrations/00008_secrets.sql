-- +goose Up
CREATE TABLE secrets (
  ref        TEXT  PRIMARY KEY,
  ciphertext BYTEA NOT NULL,
  nonce      BYTEA NOT NULL,
  key_id     TEXT  NOT NULL,
  metadata   JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE secrets;

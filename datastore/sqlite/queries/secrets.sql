-- name: WriteSecret :exec
INSERT INTO secrets (tenant_id, ref, ciphertext, nonce, key_id, metadata, created_at, updated_at)
VALUES (@tenant_id, @ref, @ciphertext, @nonce, @key_id, @metadata, @created_at, @updated_at)
ON CONFLICT (tenant_id, ref) DO UPDATE SET
    ciphertext = excluded.ciphertext,
    nonce = excluded.nonce,
    key_id = excluded.key_id,
    metadata = excluded.metadata,
    updated_at = excluded.updated_at;

-- name: ReadSecret :one
SELECT ciphertext, nonce, metadata FROM secrets WHERE ref = @ref;

-- name: DeleteSecret :exec
DELETE FROM secrets WHERE ref = @ref;

-- name: RotateSecret :execrows
UPDATE secrets SET ciphertext = @ciphertext, nonce = @nonce, key_id = @key_id,
    metadata = @metadata, updated_at = @updated_at
WHERE tenant_id = @tenant_id AND ref = @ref;

-- name: WriteSecret :exec
INSERT INTO secrets (tenant_id, ref, ciphertext, nonce, key_id, metadata, updated_at)
VALUES (@tenant_id, @ref, @ciphertext, @nonce, @key_id, @metadata, now())
ON CONFLICT (tenant_id, ref) DO UPDATE SET
    ciphertext = EXCLUDED.ciphertext,
    nonce = EXCLUDED.nonce,
    key_id = EXCLUDED.key_id,
    metadata = EXCLUDED.metadata,
    updated_at = now();

-- name: ReadSecret :one
SELECT ciphertext, nonce, metadata FROM secrets WHERE ref = @ref;

-- name: DeleteSecret :exec
DELETE FROM secrets WHERE ref = @ref;

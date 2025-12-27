-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens(
    created_at, updated_at,  token, user_id, expires_at
) VALUES (now(), now(), $1, $2, $3 ) RETURNING *;

-- name: GetRefreshToken :one

SELECT token from refresh_tokens where expires is null and token=$1;

-- name: GetUserFromRefreshToken :one
SELECT user_id from refresh_tokens 
where token=$1
and revoked_at is null;

-- name: RevokeRefreshToken :exec
UPDATE refresh_tokens SET revoked_at=now() where token=$1;
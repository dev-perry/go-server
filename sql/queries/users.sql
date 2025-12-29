-- name: CreateUser :one
INSERT INTO
    users (id, created_at, updated_at, email, hashed_password)
VALUES
    (gen_random_uuid(), now(), now(), $1, $2) RETURNING id, created_at, updated_at, email, is_chirpy_red;

-- name: GetUserCredsByEmail :one
SELECT id, email, created_at, updated_at, hashed_password, is_chirpy_red FROM users where email=$1;

-- name: DeleteAllUsers :exec
TRUNCATE users CASCADE;

-- name: UpdateUserCredentials :one
UPDATE users SET hashed_password=$1, email=$2, updated_at=now() WHERE id=$3 RETURNING id, updated_at, email, is_chirpy_red;

-- name: UpgradeUser :exec
UPDATE users SET is_chirpy_red=true WHERE id=$1 RETURNING id, is_chirpy_red;
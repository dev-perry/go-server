-- name: CreateUser :one
INSERT INTO
    users (id, created_at, updated_at, email, hashed_password)
VALUES
    (gen_random_uuid(), now(), now(), $1, $2) RETURNING id, created_at, updated_at, email;

-- name: GetUserCredsByEmail :one
SELECT id, email, created_at, updated_at, hashed_password FROM users where email=$1;

-- name: DeleteAllUsers :exec
TRUNCATE users CASCADE;

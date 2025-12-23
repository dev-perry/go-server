-- name: CreateChirp :one
INSERT INTO
    chirps (id, created_at, updated_at, body, user_id)
VALUES
    (gen_random_uuid(), now(), now(), $1, $2) RETURNING *;

-- name: GetChirp :one
SELECT * FROM chirps WHERE id = $1;

-- name: GetAllChirps :many
SELECT * FROM chirps;

-- name: DeleteAllChirps :exec
TRUNCATE chirps;
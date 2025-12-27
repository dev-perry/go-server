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

-- name: IsChirpAuthor :one
SELECT id, CASE
    WHEN user_id=$1 THEN true
    ELSE false
END AS is_author
FROM chirps
WHERE id=$2;

-- name: DeleteChirp :exec
DELETE FROM chirps WHERE id=$1 AND user_id=$2;
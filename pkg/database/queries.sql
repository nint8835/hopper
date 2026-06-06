-- name: CreateFeed :one
INSERT INTO feeds (title, description, url, feed_url, channel_id)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: DeleteFeed :exec
DELETE FROM feeds
WHERE id = ?;

-- name: GetFeeds :many
SELECT *
FROM feeds;

-- name: GetFeedByID :one
SELECT *
FROM feeds
WHERE id = ?;

-- name: GetFeedByUrl :one
SELECT *
FROM feeds
WHERE feed_url = ?;

-- name: PauseFeed :exec
UPDATE feeds
SET paused_until = ?
WHERE id = ?;

-- name: UnpauseFeed :exec
UPDATE feeds
SET paused_until = NULL
WHERE id = ?;

-- name: GetPosts :many
SELECT post_guid
FROM posts
WHERE feed_id = ?;

-- name: CreatePost :exec
INSERT INTO posts (
        post_guid,
        feed_id,
        title,
        description,
        url,
        message_id
    )
VALUES (?, ?, ?, ?, ?, ?);

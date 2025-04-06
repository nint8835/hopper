-- name: CreateFeed :one
INSERT INTO feeds (title, description, url, feed_url)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: GetFeeds :many
SELECT *
FROM feeds;

-- name: GetFeedByUrl :one
SELECT *
FROM feeds
WHERE feed_url = ?;

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

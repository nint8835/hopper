-- name: CreateFeed :one
INSERT INTO feeds (title, description, url, feed_url)
VALUES (?, ?, ?, ?)
RETURNING *;

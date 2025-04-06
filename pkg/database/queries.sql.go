// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.28.0
// source: queries.sql

package database

import (
	"context"
)

const createFeed = `-- name: CreateFeed :one
INSERT INTO feeds (title, description, url, feed_url)
VALUES (?, ?, ?, ?)
RETURNING id, title, description, url, feed_url
`

type CreateFeedParams struct {
	Title       string
	Description string
	Url         string
	FeedUrl     string
}

func (q *Queries) CreateFeed(ctx context.Context, arg CreateFeedParams) (Feed, error) {
	row := q.db.QueryRowContext(ctx, createFeed,
		arg.Title,
		arg.Description,
		arg.Url,
		arg.FeedUrl,
	)
	var i Feed
	err := row.Scan(
		&i.ID,
		&i.Title,
		&i.Description,
		&i.Url,
		&i.FeedUrl,
	)
	return i, err
}

const createPost = `-- name: CreatePost :exec
INSERT INTO posts (
        post_guid,
        feed_id,
        title,
        description,
        url,
        message_id
    )
VALUES (?, ?, ?, ?, ?, ?)
`

type CreatePostParams struct {
	PostGuid    string
	FeedID      int64
	Title       string
	Description string
	Url         string
	MessageID   string
}

func (q *Queries) CreatePost(ctx context.Context, arg CreatePostParams) error {
	_, err := q.db.ExecContext(ctx, createPost,
		arg.PostGuid,
		arg.FeedID,
		arg.Title,
		arg.Description,
		arg.Url,
		arg.MessageID,
	)
	return err
}

const getFeeds = `-- name: GetFeeds :many
SELECT id, title, description, url, feed_url
FROM feeds
`

func (q *Queries) GetFeeds(ctx context.Context) ([]Feed, error) {
	rows, err := q.db.QueryContext(ctx, getFeeds)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Feed
	for rows.Next() {
		var i Feed
		if err := rows.Scan(
			&i.ID,
			&i.Title,
			&i.Description,
			&i.Url,
			&i.FeedUrl,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getPosts = `-- name: GetPosts :many
SELECT post_guid
FROM posts
WHERE feed_id = ?
`

func (q *Queries) GetPosts(ctx context.Context, feedID int64) ([]string, error) {
	rows, err := q.db.QueryContext(ctx, getPosts, feedID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []string
	for rows.Next() {
		var post_guid string
		if err := rows.Scan(&post_guid); err != nil {
			return nil, err
		}
		items = append(items, post_guid)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

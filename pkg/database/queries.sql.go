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

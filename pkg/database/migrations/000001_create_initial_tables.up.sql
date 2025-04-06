CREATE TABLE feeds (
    id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    url TEXT NOT NULL,
    feed_url TEXT NOT NULL
);

CREATE TABLE posts (
    post_guid TEXT NOT NULL,
    feed_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    url TEXT NOT NULL,
    message_id TEXT NOT NULL,
    PRIMARY KEY (post_guid, feed_id),
    FOREIGN KEY (feed_id) REFERENCES feeds(id) ON DELETE CASCADE
);

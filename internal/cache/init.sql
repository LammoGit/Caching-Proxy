CREATE TABLE IF NOT EXISTS Pages (
		url TEXT NOT NULL,
		method TEXT NOT NULL,
		status_code INT NOT NULL,
		headers BLOB NOT NULL,
		content BLOB NOT NULL,
		etag TEXT NOT NULL,
		last_modified TEXT NOT NULL,
		updated_at INT NOT NULL DEFAULT (unixepoch()),
		PRIMARY KEY(url, method)
) WITHOUT ROWID;

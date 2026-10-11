CREATE TABLE IF NOT EXISTS Pages (
		id INTEGER PRIMARY KEY,
		url TEXT NOT NULL,
		method TEXT NOT NULL,
		headers BLOB,
		content BLOB,
		UNIQUE(url, method),
		CHECK(method IN ('GET', 'HEAD', 'OPTIONS', 'QUERY')) -- Cacheable methods
);

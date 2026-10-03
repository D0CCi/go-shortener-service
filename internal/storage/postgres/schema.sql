-- url_hash is sha256(url), computed in Go. A unique index on url itself is not possible:
-- a B-tree entry must fit ~2.7 KB, and a url may be up to 4096 characters.
CREATE TABLE IF NOT EXISTS links (
    code       varchar(10) PRIMARY KEY,
    url        text        NOT NULL,
    url_hash   bytea       NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now()
);

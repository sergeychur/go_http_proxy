DROP TABLE IF EXISTS requests;

CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE requests (
    req_id serial CONSTRAINT requests_first_key PRIMARY KEY,
    is_https bool,
    data bytea
);

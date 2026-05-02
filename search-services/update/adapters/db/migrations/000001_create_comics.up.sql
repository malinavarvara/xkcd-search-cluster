CREATE TABLE comics (
  id SERIAL PRIMARY KEY,
  num integer UNIQUE NOT NULL,
  img_url text NOT NULL,
  published_at date NOT NULL
);

CREATE TABLE words (
  id SERIAL PRIMARY KEY,
  word text UNIQUE NOT NULL
);

CREATE TABLE comic_words (
  id SERIAL PRIMARY KEY,
  comic_id integer NOT NULL,
  word_id integer NOT NULL
);

CREATE INDEX ON comic_words (comic_id);

CREATE INDEX ON comic_words (word_id);

ALTER TABLE comic_words ADD FOREIGN KEY (comic_id) REFERENCES comics (id) DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE comic_words ADD FOREIGN KEY (word_id) REFERENCES words (id) DEFERRABLE INITIALLY IMMEDIATE;
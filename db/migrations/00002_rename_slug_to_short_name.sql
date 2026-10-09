-- +goose Up
ALTER TABLE links RENAME COLUMN slug TO short_name;

-- +goose Down
ALTER TABLE links RENAME COLUMN short_name TO slug;

-- +goose Up
CREATE TABLE mygo_framework_settings (
    name TEXT PRIMARY KEY,
    value_json BLOB NOT NULL
);

-- +goose Down
DROP TABLE mygo_framework_settings;

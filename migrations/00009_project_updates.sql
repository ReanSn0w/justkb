-- +goose Up
CREATE TABLE project_update_generation (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    generation TEXT NOT NULL
);
INSERT INTO project_update_generation VALUES (1, lower(hex(randomblob(16))));
CREATE TABLE project_updates (
    sequence INTEGER PRIMARY KEY AUTOINCREMENT,
    id TEXT NOT NULL UNIQUE,
    payload TEXT NOT NULL CHECK (json_valid(payload))
);
CREATE TABLE project_update_links (
    project_id TEXT NOT NULL,
    event_sequence INTEGER NOT NULL REFERENCES project_updates(sequence),
    PRIMARY KEY (project_id, event_sequence)
);

-- +goose Down
DROP TABLE project_update_links;
DROP TABLE project_updates;
DROP TABLE project_update_generation;

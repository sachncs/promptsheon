-- Durable per-release environment overlays.
CREATE TABLE release_overlays (
    release_id    TEXT NOT NULL,
    environment   TEXT NOT NULL,
    patch_json    TEXT NOT NULL,
    updated_at    TEXT NOT NULL,
    PRIMARY KEY (release_id, environment),
    FOREIGN KEY (release_id) REFERENCES releases(id) ON DELETE CASCADE
);

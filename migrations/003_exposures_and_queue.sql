-- +goose Up

CREATE TABLE IF NOT EXISTS concept_exposures (
    id TEXT PRIMARY KEY,
    topic_id TEXT NOT NULL REFERENCES topics(id),
    source TEXT NOT NULL CHECK(source IN ('SESSION', 'AUTHORED', 'AI_ACCEPTED')),
    commit_sha TEXT,
    file_path TEXT,
    code_snippet TEXT,
    confidence REAL NOT NULL CHECK(confidence BETWEEN 0.0 AND 1.0),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE INDEX idx_exposures_topic ON concept_exposures(topic_id);
CREATE INDEX idx_exposures_created ON concept_exposures(created_at);

CREATE TABLE IF NOT EXISTS lesson_queue (
    topic_id TEXT PRIMARY KEY REFERENCES topics(id),
    priority_score REAL NOT NULL DEFAULT 0.0,
    source_exposure_id TEXT REFERENCES concept_exposures(id),
    queued_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

ALTER TABLE topic_progress ADD COLUMN ai_exposure_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE topic_progress ADD COLUMN authored_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE topic_progress ADD COLUMN last_exposure_source TEXT;
ALTER TABLE topic_progress ADD COLUMN confidence_modifier REAL NOT NULL DEFAULT 1.0;
ALTER TABLE topic_progress ADD COLUMN last_confirmed_at TEXT;

-- +goose Down

ALTER TABLE topic_progress DROP COLUMN ai_exposure_count;
ALTER TABLE topic_progress DROP COLUMN authored_count;
ALTER TABLE topic_progress DROP COLUMN last_exposure_source;
ALTER TABLE topic_progress DROP COLUMN confidence_modifier;
ALTER TABLE topic_progress DROP COLUMN last_confirmed_at;

DROP TABLE IF EXISTS lesson_queue;
DROP TABLE IF EXISTS concept_exposures;

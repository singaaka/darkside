-- +goose Up
-- +goose NO TRANSACTION

CREATE TABLE nodes (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    public_ip TEXT NOT NULL,
    ssh_user TEXT NOT NULL,
    ssh_key_path TEXT NOT NULL,
    tags TEXT NOT NULL DEFAULT '[]',     -- JSON array of strings
    is_darkside_paas_node INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'pending',
    nomad_node_id TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE cluster_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    nomad_hcl TEXT NOT NULL DEFAULT '',
    consul_hcl TEXT NOT NULL DEFAULT '',
    traefik_yml TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(node_id)
);

CREATE TABLE jobs (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    payload TEXT NOT NULL DEFAULT '{}',
    output TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at TIMESTAMP,
    finished_at TIMESTAMP
);

CREATE INDEX idx_jobs_status ON jobs(status, created_at);

CREATE TABLE settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- Seed defaults.
INSERT INTO settings (key, value) VALUES
    ('domain', ''),
    ('registry_port', '5000'),
    ('darkside_paas_node_id', '');

-- +goose Down
DROP INDEX IF EXISTS idx_jobs_status;
DROP TABLE IF EXISTS settings;
DROP TABLE IF EXISTS jobs;
DROP TABLE IF EXISTS cluster_configs;
DROP TABLE IF EXISTS nodes;

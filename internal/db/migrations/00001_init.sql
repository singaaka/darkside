-- +goose Up
-- +goose NO TRANSACTION
-- Single-statement-per-line so goose splits cleanly. SQLite is fine with
-- IF NOT EXISTS, but we leave it off here since goose's version table makes
-- duplicate runs impossible — the existing one was a hold-over from the
-- pre-migration tool boot path.

CREATE TABLE github_app (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    app_id INTEGER NOT NULL,
    slug TEXT NOT NULL,
    name TEXT NOT NULL,
    client_id TEXT NOT NULL,
    client_secret TEXT NOT NULL,
    webhook_secret TEXT NOT NULL,
    private_key TEXT NOT NULL,
    html_url TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE github_installation (
    id INTEGER PRIMARY KEY,
    account_login TEXT NOT NULL,
    account_type TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE app (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    repo_full_name TEXT NOT NULL,
    installation_id INTEGER NOT NULL REFERENCES github_installation(id),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE environment (
    id TEXT PRIMARY KEY,
    app_id TEXT NOT NULL REFERENCES app(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    age_public_key TEXT NOT NULL,
    age_private_key TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(app_id, name)
);

CREATE TABLE deployment (
    id TEXT PRIMARY KEY,
    app_id TEXT NOT NULL REFERENCES app(id) ON DELETE CASCADE,
    environment_id TEXT REFERENCES environment(id),
    commit_sha TEXT NOT NULL,
    commit_message TEXT NOT NULL,
    image_tag TEXT,
    status TEXT NOT NULL,
    nomad_job_hcl TEXT,
    env_snapshot TEXT,
    error TEXT,
    started_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    finished_at TIMESTAMP
);

CREATE TABLE deployment_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    deployment_id TEXT NOT NULL REFERENCES deployment(id) ON DELETE CASCADE,
    phase TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(deployment_id, phase)
);

CREATE INDEX idx_deployment_app ON deployment(app_id, started_at DESC);
CREATE INDEX idx_deployment_env ON deployment(environment_id, started_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_deployment_env;
DROP INDEX IF EXISTS idx_deployment_app;
DROP TABLE IF EXISTS deployment_log;
DROP TABLE IF EXISTS deployment;
DROP TABLE IF EXISTS environment;
DROP TABLE IF EXISTS app;
DROP TABLE IF EXISTS github_installation;
DROP TABLE IF EXISTS github_app;

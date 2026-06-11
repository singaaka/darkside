-- +goose Up
-- +goose NO TRANSACTION

-- Disable FK checks for this migration so we can restructure tables that
-- reference each other.
PRAGMA foreign_keys = OFF;

-- ── App: add per-app age key + deploy branch ────────────────────────────────
ALTER TABLE app ADD COLUMN branch TEXT NOT NULL DEFAULT 'main';
ALTER TABLE app ADD COLUMN env_file TEXT NOT NULL DEFAULT 'env.age';
ALTER TABLE app ADD COLUMN age_public_key TEXT NOT NULL DEFAULT '';
ALTER TABLE app ADD COLUMN age_private_key TEXT NOT NULL DEFAULT '';
ALTER TABLE app ADD COLUMN age_key_id TEXT NOT NULL DEFAULT 'key-v1';

-- ── Users & sessions ────────────────────────────────────────────────────────
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    github_id INTEGER UNIQUE NOT NULL,
    login TEXT NOT NULL,
    email TEXT,
    is_admin INTEGER NOT NULL DEFAULT 0,
    whitelisted INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE sessions (
    token TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL
);

-- ── Deployment: add trigger metadata + rendered job HCLs ────────────────────
ALTER TABLE deployment ADD COLUMN trigger_type TEXT NOT NULL DEFAULT 'github';
ALTER TABLE deployment ADD COLUMN trigger_branch TEXT;
ALTER TABLE deployment ADD COLUMN build_job_hcl TEXT;
ALTER TABLE deployment ADD COLUMN pre_hook_hcl TEXT;
ALTER TABLE deployment ADD COLUMN post_hook_hcl TEXT;

-- Drop env index and environment_id column (environment concept removed).
DROP INDEX IF EXISTS idx_deployment_env;
ALTER TABLE deployment DROP COLUMN environment_id;

-- Drop the environment table — replaced by per-app age key on the app row.
DROP TABLE IF EXISTS environment;

PRAGMA foreign_keys = ON;

-- +goose Down
-- +goose NO TRANSACTION

PRAGMA foreign_keys = OFF;

CREATE TABLE environment (
    id TEXT PRIMARY KEY,
    app_id TEXT NOT NULL REFERENCES app(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    age_public_key TEXT NOT NULL,
    age_private_key TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(app_id, name)
);

ALTER TABLE deployment ADD COLUMN environment_id TEXT REFERENCES environment(id);
CREATE INDEX idx_deployment_env ON deployment(environment_id, started_at DESC);

ALTER TABLE deployment DROP COLUMN post_hook_hcl;
ALTER TABLE deployment DROP COLUMN pre_hook_hcl;
ALTER TABLE deployment DROP COLUMN build_job_hcl;
ALTER TABLE deployment DROP COLUMN trigger_branch;
ALTER TABLE deployment DROP COLUMN trigger_type;

DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;

ALTER TABLE app DROP COLUMN age_key_id;
ALTER TABLE app DROP COLUMN age_private_key;
ALTER TABLE app DROP COLUMN age_public_key;
ALTER TABLE app DROP COLUMN env_file;
ALTER TABLE app DROP COLUMN branch;

PRAGMA foreign_keys = ON;

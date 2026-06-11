-- name: GetGitHubApp :one
SELECT * FROM github_app WHERE id = 1;

-- name: UpsertGitHubApp :exec
INSERT INTO github_app (id, app_id, slug, name, client_id, client_secret, webhook_secret, private_key, html_url)
VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    app_id = excluded.app_id,
    slug = excluded.slug,
    name = excluded.name,
    client_id = excluded.client_id,
    client_secret = excluded.client_secret,
    webhook_secret = excluded.webhook_secret,
    private_key = excluded.private_key,
    html_url = excluded.html_url;

-- name: ListInstallations :many
SELECT * FROM github_installation ORDER BY account_login;

-- name: UpsertInstallation :exec
INSERT INTO github_installation (id, account_login, account_type)
VALUES (?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    account_login = excluded.account_login,
    account_type = excluded.account_type;

-- name: ListApps :many
SELECT * FROM app ORDER BY name;

-- name: GetApp :one
SELECT * FROM app WHERE id = ?;

-- name: GetAppByName :one
SELECT * FROM app WHERE name = ?;

-- name: GetAppByRepo :one
SELECT * FROM app WHERE repo_full_name = ?;

-- name: CreateApp :exec
INSERT INTO app (id, name, repo_full_name, installation_id, branch, env_file, age_public_key, age_private_key, age_key_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateAppAgeKey :exec
UPDATE app SET age_public_key = ?, age_private_key = ?, age_key_id = ? WHERE id = ?;

-- name: ListDeployments :many
SELECT * FROM deployment WHERE app_id = ? ORDER BY started_at DESC LIMIT ?;

-- name: GetDeployment :one
SELECT * FROM deployment WHERE id = ?;

-- name: CreateDeployment :exec
INSERT INTO deployment (id, app_id, commit_sha, commit_message, status, trigger_type, trigger_branch)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: UpdateDeploymentStatus :exec
UPDATE deployment SET status = ?, error = ? WHERE id = ?;

-- name: UpdateDeploymentArtifact :exec
UPDATE deployment SET image_tag = ?, nomad_job_hcl = ?, env_snapshot = ?,
    build_job_hcl = ?, pre_hook_hcl = ?, post_hook_hcl = ? WHERE id = ?;

-- name: FinishDeployment :exec
UPDATE deployment SET status = ?, finished_at = CURRENT_TIMESTAMP WHERE id = ?;

-- name: UpsertDeploymentLog :exec
INSERT INTO deployment_log (deployment_id, phase, content)
VALUES (?, ?, ?)
ON CONFLICT(deployment_id, phase) DO UPDATE SET content = excluded.content;

-- name: GetDeploymentLog :one
SELECT * FROM deployment_log WHERE deployment_id = ? AND phase = ?;

-- name: UpsertUser :one
INSERT INTO users (github_id, login, email, is_admin, whitelisted)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(github_id) DO UPDATE SET
    login = excluded.login,
    email = excluded.email
RETURNING *;

-- name: GetUserByGitHubID :one
SELECT * FROM users WHERE github_id = ?;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = ?;

-- name: CountUsers :one
SELECT COUNT(*) FROM users;

-- name: ListUsers :many
SELECT * FROM users ORDER BY created_at;

-- name: WhitelistUser :exec
UPDATE users SET whitelisted = 1 WHERE id = ?;

-- name: CreateSession :exec
INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?);

-- name: GetSession :one
SELECT s.*, u.id as uid, u.github_id, u.login, u.email, u.is_admin, u.whitelisted
FROM sessions s JOIN users u ON u.id = s.user_id
WHERE s.token = ? AND s.expires_at > CURRENT_TIMESTAMP;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE token = ?;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions WHERE expires_at <= CURRENT_TIMESTAMP;

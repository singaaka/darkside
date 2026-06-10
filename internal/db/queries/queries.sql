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
INSERT INTO app (id, name, repo_full_name, installation_id) VALUES (?, ?, ?, ?);

-- name: ListEnvironments :many
SELECT * FROM environment WHERE app_id = ? ORDER BY name;

-- name: GetEnvironment :one
SELECT * FROM environment WHERE app_id = ? AND name = ?;

-- name: GetEnvironmentByID :one
SELECT * FROM environment WHERE id = ?;

-- name: CreateEnvironment :exec
INSERT INTO environment (id, app_id, name, age_public_key, age_private_key)
VALUES (?, ?, ?, ?, ?);

-- name: ListDeployments :many
SELECT * FROM deployment WHERE app_id = ? ORDER BY started_at DESC LIMIT ?;

-- name: GetDeployment :one
SELECT * FROM deployment WHERE id = ?;

-- name: CreateDeployment :exec
INSERT INTO deployment (id, app_id, environment_id, commit_sha, commit_message, status)
VALUES (?, ?, ?, ?, ?, ?);

-- name: UpdateDeploymentStatus :exec
UPDATE deployment SET status = ?, error = ? WHERE id = ?;

-- name: UpdateDeploymentArtifact :exec
UPDATE deployment SET image_tag = ?, nomad_job_hcl = ?, env_snapshot = ? WHERE id = ?;

-- name: UpdateDeploymentEnvironment :exec
UPDATE deployment SET environment_id = ? WHERE id = ?;

-- name: FinishDeployment :exec
UPDATE deployment SET status = ?, finished_at = CURRENT_TIMESTAMP WHERE id = ?;

-- name: UpsertDeploymentLog :exec
INSERT INTO deployment_log (deployment_id, phase, content)
VALUES (?, ?, ?)
ON CONFLICT(deployment_id, phase) DO UPDATE SET content = excluded.content;

-- name: GetDeploymentLog :one
SELECT * FROM deployment_log WHERE deployment_id = ? AND phase = ?;

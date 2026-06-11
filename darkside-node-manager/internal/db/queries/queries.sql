-- name: ListNodes :many
SELECT * FROM nodes ORDER BY created_at;

-- name: GetNode :one
SELECT * FROM nodes WHERE id = ?;

-- name: GetNodeByName :one
SELECT * FROM nodes WHERE name = ?;

-- name: CreateNode :exec
INSERT INTO nodes (id, name, public_ip, ssh_user, ssh_key_path, tags, is_darkside_paas_node)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: UpdateNodeStatus :exec
UPDATE nodes SET status = ?, nomad_node_id = COALESCE(?, nomad_node_id) WHERE id = ?;

-- name: SetPaasNode :exec
UPDATE nodes SET is_darkside_paas_node = (id = ?);

-- name: DeleteNode :exec
DELETE FROM nodes WHERE id = ?;

-- name: CountNodes :one
SELECT COUNT(*) FROM nodes;

-- name: UpsertClusterConfig :exec
INSERT INTO cluster_configs (node_id, nomad_hcl, consul_hcl, traefik_yml, updated_at)
VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
ON CONFLICT(node_id) DO UPDATE SET
    nomad_hcl = excluded.nomad_hcl,
    consul_hcl = excluded.consul_hcl,
    traefik_yml = excluded.traefik_yml,
    updated_at = excluded.updated_at;

-- name: GetClusterConfig :one
SELECT * FROM cluster_configs WHERE node_id = ?;

-- name: ListJobs :many
SELECT * FROM jobs ORDER BY created_at DESC LIMIT ?;

-- name: GetJob :one
SELECT * FROM jobs WHERE id = ?;

-- name: GetPendingJob :one
SELECT * FROM jobs WHERE status = 'pending' ORDER BY created_at LIMIT 1;

-- name: CreateJob :exec
INSERT INTO jobs (id, type, status, payload) VALUES (?, ?, 'pending', ?);

-- name: StartJob :exec
UPDATE jobs SET status = 'running', started_at = CURRENT_TIMESTAMP WHERE id = ?;

-- name: AppendJobOutput :exec
UPDATE jobs SET output = output || ? WHERE id = ?;

-- name: FinishJob :exec
UPDATE jobs SET status = ?, finished_at = CURRENT_TIMESTAMP WHERE id = ?;

-- name: GetSetting :one
SELECT value FROM settings WHERE key = ?;

-- name: SetSetting :exec
INSERT INTO settings (key, value) VALUES (?, ?)
ON CONFLICT(key) DO UPDATE SET value = excluded.value;

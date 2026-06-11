// Typed fetch helpers for the /api/v1/ REST endpoints.
// These shapes deliberately live in TS rather than the proto — they're
// JSON-only handlers and don't need cross-language codegen.

async function apiFetch<T>(path: string, opts?: RequestInit): Promise<T> {
  const r = await fetch(path, {
    headers: { "Content-Type": "application/json" },
    ...opts,
  })
  if (!r.ok) {
    const body = await r.json().catch(() => ({ error: r.statusText }))
    throw new Error(body.error ?? r.statusText)
  }
  return r.json()
}

// ── REST response shapes ─────────────────────────────────────────────────────

export interface AppDetail {
  id: string
  name: string
  repo_full_name: string
  installation_id: number
  created_at: number
  branch: string
  env_file: string
  age_public_key: string
  age_key_id: string
  // Only present in the response to POST /api/v1/apps (creation).
  age_private_key?: string
  setup_instructions?: string
}

export interface DeploymentDetail {
  id: string
  app_id: string
  commit_sha: string
  commit_message: string
  status: string
  trigger_type: string
  started_at: number
  finished_at?: number
  image_tag?: string
  trigger_branch?: string
  nomad_job_hcl?: string
  build_job_hcl?: string
  pre_hook_hcl?: string
  post_hook_hcl?: string
  env_snapshot?: string
  error?: string
}

export interface UserProfile {
  id: number
  login: string
  email?: string
  is_admin: boolean
  whitelisted: boolean
}

// OperatorConfig is what `darkside.toml` resolves to at runtime — surfaced
// here so operators can see what the binary actually picked up.
export interface OperatorConfig {
  external_url: string
  host: string
  listen: string
  data_dir: string
  nomad_addr: string
  registry_addr: string
  docker_host: string
  version: string
}

// ── Apps ─────────────────────────────────────────────────────────────────────

export function getAppDetail(id: string): Promise<AppDetail> {
  return apiFetch(`/api/v1/apps/${id}`)
}

export function createApp(body: {
  name: string
  repo_full_name: string
  installation_id: bigint
  branch?: string
  env_file?: string
}): Promise<AppDetail> {
  return apiFetch("/api/v1/apps", {
    method: "POST",
    body: JSON.stringify({ ...body, installation_id: Number(body.installation_id) }),
  })
}

export function rotateKey(appId: string): Promise<{
  age_key_id: string
  age_public_key: string
  age_private_key: string
  setup_instructions: string
  toml_snippet: string
}> {
  return apiFetch(`/api/v1/apps/${appId}/rotate-key`, { method: "POST" })
}

export function scaleApp(appId: string, count: number): Promise<{ ok: boolean; count: number; warning: string }> {
  return apiFetch(`/api/v1/apps/${appId}/scale`, {
    method: "POST",
    body: JSON.stringify({ count }),
  })
}

export function manualDeploy(appId: string, branch?: string, commitSha?: string): Promise<DeploymentDetail> {
  return apiFetch(`/api/v1/apps/${appId}/deploy`, {
    method: "POST",
    body: JSON.stringify({ branch, commit_sha: commitSha }),
  })
}

// ── Deployments ──────────────────────────────────────────────────────────────

export function getDeploymentDetail(id: string): Promise<DeploymentDetail> {
  return apiFetch(`/api/v1/deployments/${id}`)
}

export function redeployFromJSON(deploymentId: string): Promise<DeploymentDetail> {
  return apiFetch(`/api/v1/deployments/${deploymentId}/redeploy`, { method: "POST" })
}

// ── Users ────────────────────────────────────────────────────────────────────

export function listUsers(): Promise<UserProfile[]> {
  return apiFetch("/api/v1/users")
}

export function whitelistUser(userId: number): Promise<{ ok: boolean }> {
  return apiFetch(`/api/v1/users/${userId}/whitelist`, { method: "POST" })
}

// ── Operator config ──────────────────────────────────────────────────────────

export function getOperatorConfig(): Promise<OperatorConfig> {
  return apiFetch("/api/v1/operator-config")
}

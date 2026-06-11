// Typed fetch helpers for the /api/v1/ REST endpoints.

import type { AppDetail, DeploymentDetail, UserProfile } from "@/gen/darkside/v1/api_pb"

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

// ── Users ─────────────────────────────────────────────────────────────────────

export function listUsers(): Promise<UserProfile[]> {
  return apiFetch("/api/v1/users")
}

export function whitelistUser(userId: number): Promise<{ ok: boolean }> {
  return apiFetch(`/api/v1/users/${userId}/whitelist`, { method: "POST" })
}

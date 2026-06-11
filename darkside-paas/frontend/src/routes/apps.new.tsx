import { useMemo, useState } from "react"
import { createFileRoute, useNavigate } from "@tanstack/react-router"
import { useMutation, useQuery } from "@connectrpc/connect-query"
import { AppService, GitHubService } from "@/gen/darkside/v1/api_pb"
import { Button } from "@/components/ui/button"

export const Route = createFileRoute("/apps/new")({
  component: NewApp,
})

function suggestSlug(repoFullName: string): string {
  const after = repoFullName.split("/")[1] ?? repoFullName
  return after
    .toLowerCase()
    .replace(/[^a-z0-9-]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 38)
}

function NewApp() {
  const navigate = useNavigate()
  const installations = useQuery(GitHubService.method.listInstallations)
  const [installationId, setInstallationId] = useState<bigint | null>(null)

  const repos = useQuery(
    GitHubService.method.listInstallationRepos,
    { installationId: installationId ?? 0n },
    { enabled: installationId !== null },
  )

  const [repoFullName, setRepoFullName] = useState("")
  const [name, setName] = useState("")
  const suggested = useMemo(() => (repoFullName ? suggestSlug(repoFullName) : ""), [repoFullName])

  const create = useMutation(AppService.method.create, {
    onSuccess: (app) => navigate({ to: "/apps/$appId", params: { appId: app.id } }),
  })

  const submit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!installationId || !repoFullName) return
    create.mutate({
      name: name.trim() || suggested,
      repoFullName,
      installationId,
    })
  }

  return (
    <form onSubmit={submit} className="space-y-6 max-w-2xl">
      <div>
        <h1 className="text-2xl font-semibold">New app</h1>
        <p className="text-muted-foreground mt-1">
          Pick a repo from a GitHub installation. You&apos;ll add the <code>darkside.toml</code>{" "}
          on the next screen.
        </p>
      </div>

      <Field
        label="Installation"
        hint="Where the darkside GitHub App is installed (org or user)."
      >
        <select
          value={installationId !== null ? String(installationId) : ""}
          onChange={(e) => setInstallationId(e.target.value ? BigInt(e.target.value) : null)}
          className="w-full rounded-md border bg-background px-3 py-2 text-sm"
          required
        >
          <option value="">Select installation…</option>
          {installations.data?.installations.map((i) => (
            <option key={String(i.id)} value={String(i.id)}>
              {i.accountLogin} ({i.accountType})
            </option>
          ))}
        </select>
        {installations.error ? (
          <p className="mt-1 text-xs text-destructive">{installations.error.message}</p>
        ) : null}
      </Field>

      <Field label="Repository" hint="Only repos this installation has access to.">
        <select
          value={repoFullName}
          onChange={(e) => setRepoFullName(e.target.value)}
          disabled={!installationId || repos.isLoading}
          className="w-full rounded-md border bg-background px-3 py-2 text-sm disabled:opacity-50"
          required
        >
          <option value="">
            {!installationId ? "Pick an installation first" : repos.isLoading ? "Loading…" : "Select repo…"}
          </option>
          {repos.data?.repos.map((r) => (
            <option key={String(r.id)} value={r.fullName}>
              {r.fullName} {r.private ? "(private)" : ""}
            </option>
          ))}
        </select>
      </Field>

      <Field
        label="App name"
        hint="Slug used for the docker image, nomad job, and traefik labels. Lowercase, dashes ok."
      >
        <input
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder={suggested || "my-app"}
          pattern="[a-z0-9][a-z0-9-]{1,38}[a-z0-9]"
          className="w-full rounded-md border bg-background px-3 py-2 text-sm font-mono"
        />
      </Field>

      {create.error ? (
        <p className="text-sm text-destructive">Error: {create.error.message}</p>
      ) : null}

      <Button type="submit" disabled={create.isPending || !installationId || !repoFullName}>
        {create.isPending ? "Creating…" : "Create app"}
      </Button>
    </form>
  )
}

function Field({
  label,
  hint,
  children,
}: {
  label: string
  hint?: string
  children: React.ReactNode
}) {
  return (
    <label className="block">
      <span className="block text-sm font-medium">{label}</span>
      {hint ? <span className="block text-xs text-muted-foreground mt-0.5">{hint}</span> : null}
      <div className="mt-2">{children}</div>
    </label>
  )
}

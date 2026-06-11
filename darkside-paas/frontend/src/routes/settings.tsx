import { useState } from "react"
import { createFileRoute } from "@tanstack/react-router"
import { useQuery } from "@connectrpc/connect-query"
import { useQuery as useRQQuery, useMutation as useRQMutation } from "@tanstack/react-query"
import { GitHubService } from "@/gen/darkside/v1/api_pb"
import { Button } from "@/components/ui/button"
import { useAuthStore } from "@/lib/auth"
import { listUsers, whitelistUser } from "@/lib/api"

export const Route = createFileRoute("/settings")({
  component: SettingsPage,
})

type Tab = "github" | "users"

function SettingsPage() {
  const { user } = useAuthStore()
  const [tab, setTab] = useState<Tab>("github")
  const tabs: Tab[] = ["github"]
  if (user?.is_admin) tabs.push("users")

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold">Settings</h1>
      <div className="border-b">
        <nav className="-mb-px flex gap-6">
          {tabs.map((t) => (
            <button key={t} type="button" onClick={() => setTab(t)}
              className={`py-3 text-sm border-b-2 transition-colors ${tab === t ? "border-foreground text-foreground" : "border-transparent text-muted-foreground hover:text-foreground"}`}>
              {t === "github" ? "GitHub App" : "Users"}
            </button>
          ))}
        </nav>
      </div>
      {tab === "github" && <GitHubTab />}
      {tab === "users" && user?.is_admin && <UsersTab />}
    </div>
  )
}

function GitHubTab() {
  const status = useQuery(GitHubService.method.getStatus)
  const [org, setOrg] = useState("")

  return (
    <section className="space-y-3 rounded-lg border p-5">
      <h2 className="text-lg font-medium">GitHub App</h2>
      {status.isLoading ? (
        <p className="text-sm text-muted-foreground">Checking…</p>
      ) : status.data?.connected ? (
        <div className="space-y-3">
          <dl className="grid grid-cols-[max-content_1fr] gap-x-4 gap-y-1 text-sm">
            <dt className="text-muted-foreground">App name</dt><dd className="font-mono">{status.data.appName}</dd>
            <dt className="text-muted-foreground">Slug</dt><dd className="font-mono">{status.data.appSlug}</dd>
            <dt className="text-muted-foreground">App ID</dt><dd className="font-mono">{String(status.data.appId)}</dd>
          </dl>
          <Button asChild>
            <a href={status.data.installUrl} target="_blank" rel="noreferrer">Install on a repo →</a>
          </Button>
        </div>
      ) : (
        <div className="space-y-3">
          <p className="text-sm">No GitHub App connected yet.</p>
          <label className="block text-sm">
            <span className="block text-muted-foreground mb-1">Organization (optional)</span>
            <input value={org} onChange={(e) => setOrg(e.target.value)} placeholder="acme-inc"
              className="w-full max-w-sm rounded-md border bg-background px-3 py-2 text-sm font-mono" />
          </label>
          <Button onClick={() => { window.location.href = org ? `/github/manifest/start?org=${org}` : "/github/manifest/start" }}>
            Connect GitHub
          </Button>
        </div>
      )}
    </section>
  )
}

function UsersTab() {
  const users = useRQQuery({ queryKey: ["users"], queryFn: listUsers })
  const whitelist = useRQMutation({
    mutationFn: (id: number) => whitelistUser(id),
    onSuccess: () => users.refetch(),
  })

  if (users.isLoading) return <p className="text-sm text-muted-foreground">Loading…</p>

  return (
    <div className="space-y-4">
      <p className="text-sm text-muted-foreground">Users who have signed in. Whitelist them to grant access.</p>
      <ul className="divide-y rounded-lg border">
        {(users.data ?? []).map((u) => (
          <li key={u.id} className="p-4 flex items-center justify-between gap-4">
            <div>
              <div className="flex items-center gap-2 text-sm">
                <span className="font-medium">{u.login}</span>
                {u.is_admin && <span className="text-xs bg-purple-100 text-purple-700 px-1.5 py-0.5 rounded">admin</span>}
                {u.whitelisted
                  ? <span className="text-xs bg-emerald-100 text-emerald-700 px-1.5 py-0.5 rounded">whitelisted</span>
                  : <span className="text-xs bg-yellow-100 text-yellow-700 px-1.5 py-0.5 rounded">pending</span>}
              </div>
              {u.email && <p className="text-xs text-muted-foreground mt-0.5">{u.email}</p>}
            </div>
            {!u.whitelisted && (
              <Button size="sm" variant="outline" onClick={() => whitelist.mutate(u.id)} disabled={whitelist.isPending}>
                Whitelist
              </Button>
            )}
          </li>
        ))}
      </ul>
    </div>
  )
}

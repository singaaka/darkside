import { useState } from "react"
import { createFileRoute } from "@tanstack/react-router"
import { useQuery } from "@connectrpc/connect-query"
import { GitHubService } from "@/gen/darkside/v1/api_pb"
import { Button } from "@/components/ui/button"

export const Route = createFileRoute("/settings")({
  component: SettingsPage,
})

function SettingsPage() {
  const status = useQuery(GitHubService.method.getStatus)
  const [org, setOrg] = useState("")

  const startManifestFlow = () => {
    const url = org.trim()
      ? `/github/manifest/start?org=${encodeURIComponent(org.trim())}`
      : "/github/manifest/start"
    window.location.href = url
  }

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-2xl font-semibold">Settings</h1>
        <p className="text-muted-foreground mt-1">
          One GitHub App connects darkside to your repos.
        </p>
      </div>

      <section className="space-y-3 rounded-lg border p-5">
        <h2 className="text-lg font-medium">GitHub App</h2>

        {status.isLoading ? (
          <p className="text-sm text-muted-foreground">Checking…</p>
        ) : status.data?.connected ? (
          <div className="space-y-3">
            <dl className="grid grid-cols-[max-content_1fr] gap-x-4 gap-y-1 text-sm">
              <dt className="text-muted-foreground">App name</dt>
              <dd className="font-mono">{status.data.appName}</dd>
              <dt className="text-muted-foreground">Slug</dt>
              <dd className="font-mono">{status.data.appSlug}</dd>
              <dt className="text-muted-foreground">App ID</dt>
              <dd className="font-mono">{String(status.data.appId)}</dd>
              <dt className="text-muted-foreground">URL</dt>
              <dd>
                <a className="underline" href={status.data.htmlUrl} target="_blank" rel="noreferrer">
                  {status.data.htmlUrl}
                </a>
              </dd>
            </dl>
            <Button asChild>
              <a href={status.data.installUrl} target="_blank" rel="noreferrer">
                Install on a repo →
              </a>
            </Button>
          </div>
        ) : (
          <div className="space-y-3">
            <p className="text-sm">
              Create a GitHub App. You&apos;ll be redirected to github.com to confirm; on success,
              darkside stores the credentials and you can install it on the repos you want to
              deploy.
            </p>
            <label className="block text-sm">
              <span className="block text-muted-foreground mb-1">
                Install for organization (optional — leave empty for personal account)
              </span>
              <input
                value={org}
                onChange={(e) => setOrg(e.target.value)}
                placeholder="acme-inc"
                className="w-full max-w-sm rounded-md border bg-background px-3 py-2 text-sm font-mono"
              />
            </label>
            <Button onClick={startManifestFlow}>Connect GitHub</Button>
          </div>
        )}
      </section>
    </div>
  )
}

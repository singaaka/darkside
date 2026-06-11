import { createFileRoute, Link } from "@tanstack/react-router"
import { useQuery } from "@connectrpc/connect-query"
import { AppService, GitHubService } from "@/gen/darkside/v1/api_pb"
import { Button } from "@/components/ui/button"

export const Route = createFileRoute("/")({
  component: Dashboard,
})

function Dashboard() {
  const status = useQuery(GitHubService.method.getStatus)
  const apps = useQuery(AppService.method.list)

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">Dashboard</h1>
        <p className="text-muted-foreground mt-1">
          {status.data?.connected
            ? `Connected to GitHub App ${status.data.appName}.`
            : "GitHub App not yet connected — head to Settings."}
        </p>
      </div>

      <div className="grid gap-4 md:grid-cols-3">
        <div className="rounded-lg border p-4">
          <div className="text-sm text-muted-foreground">GitHub App</div>
          <div className="mt-2 text-2xl font-semibold">
            {status.isLoading ? "…" : status.data?.connected ? "connected" : "—"}
          </div>
        </div>
        <div className="rounded-lg border p-4">
          <div className="text-sm text-muted-foreground">Apps</div>
          <div className="mt-2 text-2xl font-semibold">
            {apps.isLoading ? "…" : apps.data?.apps.length ?? 0}
          </div>
        </div>
        <div className="rounded-lg border p-4">
          <div className="text-sm text-muted-foreground">Next up</div>
          <div className="mt-2 text-sm">
            {status.data?.connected ? (
              <Link to="/apps/new" className="underline">
                Add a new app →
              </Link>
            ) : (
              <Link to="/settings" className="underline">
                Connect GitHub →
              </Link>
            )}
          </div>
        </div>
      </div>

      <div className="flex gap-3">
        <Button asChild>
          <Link to="/apps">Manage apps</Link>
        </Button>
        <Button asChild variant="outline">
          <Link to="/settings">Settings</Link>
        </Button>
      </div>
    </div>
  )
}

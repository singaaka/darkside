import { createFileRoute, Link } from "@tanstack/react-router"
import { useQuery } from "@connectrpc/connect-query"
import { AppService } from "@/gen/darkside/v1/api_pb"
import { Button } from "@/components/ui/button"

export const Route = createFileRoute("/apps/")({
  component: AppsList,
})

function AppsList() {
  const apps = useQuery(AppService.method.list)

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold">Apps</h1>
          <p className="text-muted-foreground mt-1">
            One app per GitHub repo. Each app builds and deploys its own image.
          </p>
        </div>
        <Button asChild>
          <Link to="/apps/new">New app</Link>
        </Button>
      </div>

      {apps.isLoading ? (
        <p className="text-sm text-muted-foreground">Loading…</p>
      ) : apps.error ? (
        <p className="text-sm text-destructive">Error: {apps.error.message}</p>
      ) : (apps.data?.apps.length ?? 0) === 0 ? (
        <div className="rounded-lg border border-dashed p-8 text-center">
          <p className="text-sm text-muted-foreground">No apps yet.</p>
          <Button asChild className="mt-3">
            <Link to="/apps/new">Connect your first repo</Link>
          </Button>
        </div>
      ) : (
        <ul className="divide-y rounded-lg border">
          {apps.data!.apps.map((a) => (
            <li key={a.id} className="p-4 flex items-center justify-between">
              <div>
                <div className="font-medium">{a.name}</div>
                <div className="text-sm text-muted-foreground font-mono">{a.repoFullName}</div>
              </div>
              <Button asChild variant="outline" size="sm">
                <Link to="/apps/$appId" params={{ appId: a.id }}>
                  Open
                </Link>
              </Button>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

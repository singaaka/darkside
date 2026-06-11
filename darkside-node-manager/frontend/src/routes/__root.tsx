import { createRootRouteWithContext, Link, Outlet } from "@tanstack/react-router"
import { TanStackRouterDevtools } from "@tanstack/react-router-devtools"
import { ReactQueryDevtools } from "@tanstack/react-query-devtools"
import type { QueryClient } from "@tanstack/react-query"

export const Route = createRootRouteWithContext<{ queryClient: QueryClient }>()({
  component: RootLayout,
})

function RootLayout() {
  return (
    <div className="min-h-screen flex flex-col">
      <header className="border-b">
        <div className="mx-auto max-w-6xl px-6 h-14 flex items-center gap-6">
          <Link to="/" className="font-semibold tracking-tight">
            <span className="text-muted-foreground font-normal">darkside /</span> node manager
          </Link>
          <nav className="flex gap-4 text-sm text-muted-foreground">
            <Link to="/" activeProps={{ className: "text-foreground" }} activeOptions={{ exact: true }}>Nodes</Link>
            <Link to="/jobs" activeProps={{ className: "text-foreground" }}>Jobs</Link>
            <Link to="/settings" activeProps={{ className: "text-foreground" }}>Settings</Link>
          </nav>
        </div>
      </header>
      <main className="flex-1">
        <div className="mx-auto max-w-6xl px-6 py-8">
          <Outlet />
        </div>
      </main>
      {import.meta.env.DEV && (
        <>
          <TanStackRouterDevtools position="bottom-right" />
          <ReactQueryDevtools buttonPosition="bottom-left" />
        </>
      )}
    </div>
  )
}

import { createRootRouteWithContext, Link, Outlet, useNavigate, useRouterState } from "@tanstack/react-router"
import { TanStackRouterDevtools } from "@tanstack/react-router-devtools"
import { ReactQueryDevtools } from "@tanstack/react-query-devtools"
import type { QueryClient } from "@tanstack/react-query"
import { useEffect } from "react"
import { fetchMe, useAuthStore } from "@/lib/auth"

interface RouterContext {
  queryClient: QueryClient
}

export const Route = createRootRouteWithContext<RouterContext>()({
  component: RootLayout,
})

const PUBLIC_PATHS = ["/login", "/onboarding"]

function RootLayout() {
  const { user, loading, setUser, setLoading } = useAuthStore()
  const navigate = useNavigate()
  const location = useRouterState({ select: (s) => s.location.pathname })
  const isPublic = PUBLIC_PATHS.some((p) => location.startsWith(p))

  useEffect(() => {
    fetchMe().then((u) => {
      setUser(u)
      setLoading(false)
    })
  }, [setUser, setLoading])

  useEffect(() => {
    if (loading) return
    if (!user && !isPublic) navigate({ to: "/login" })
  }, [user, loading, isPublic, navigate])

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <p className="text-muted-foreground text-sm">Loading…</p>
      </div>
    )
  }

  return (
    <div className="min-h-screen flex flex-col">
      {user && !isPublic && (
        <header className="border-b">
          <div className="mx-auto max-w-6xl px-6 h-14 flex items-center gap-6">
            <Link to="/" className="font-semibold tracking-tight">darkside</Link>
            <nav className="flex gap-4 text-sm text-muted-foreground">
              <Link to="/" activeProps={{ className: "text-foreground" }} activeOptions={{ exact: true }}>Dashboard</Link>
              <Link to="/apps" activeProps={{ className: "text-foreground" }}>Apps</Link>
              <Link to="/settings" activeProps={{ className: "text-foreground" }}>Settings</Link>
            </nav>
            <div className="ml-auto flex items-center gap-3 text-sm text-muted-foreground">
              <span>{user.login}</span>
              <a href="/auth/logout" className="hover:text-foreground">Sign out</a>
            </div>
          </div>
        </header>
      )}
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

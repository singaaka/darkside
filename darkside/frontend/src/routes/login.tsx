import { createFileRoute, useSearch } from "@tanstack/react-router"
import { useAuthStore } from "@/lib/auth"
import { useEffect } from "react"
import { useNavigate } from "@tanstack/react-router"
import { Button } from "@/components/ui/button"

export const Route = createFileRoute("/login")({
  component: LoginPage,
  validateSearch: (s: Record<string, unknown>) => ({
    pending: s.pending === "1" || s.pending === true,
  }),
})

function LoginPage() {
  const { user } = useAuthStore()
  const navigate = useNavigate()
  const { pending } = useSearch({ from: "/login" })

  useEffect(() => {
    if (user) navigate({ to: "/" })
  }, [user, navigate])

  return (
    <div className="min-h-screen flex items-center justify-center">
      <div className="max-w-sm w-full space-y-6 text-center">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">darkside</h1>
          <p className="text-muted-foreground mt-2 text-sm">
            Platform as a service. Deployed by you, for you.
          </p>
        </div>

        {pending ? (
          <div className="rounded-lg border border-yellow-200 bg-yellow-50 p-4 text-sm text-yellow-800">
            Your account is pending approval. Ask an admin to whitelist your GitHub email.
          </div>
        ) : null}

        <Button asChild className="w-full">
          <a href="/auth/github">
            <svg className="mr-2 h-4 w-4" viewBox="0 0 24 24" fill="currentColor">
              <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z" />
            </svg>
            Continue with GitHub
          </a>
        </Button>

        <p className="text-xs text-muted-foreground">
          Darkside uses your GitHub account for authentication.
          The first person to sign in becomes the admin.
        </p>
      </div>
    </div>
  )
}

import { createFileRoute } from "@tanstack/react-router"
import { useState } from "react"
import { Button } from "@/components/ui/button"

export const Route = createFileRoute("/onboarding")({
  component: OnboardingPage,
})

function OnboardingPage() {
  const [org, setOrg] = useState("")

  const startGitHubAppFlow = () => {
    const url = org.trim()
      ? `/github/manifest/start?org=${encodeURIComponent(org.trim())}`
      : "/github/manifest/start"
    window.location.href = url
  }

  return (
    <div className="min-h-screen flex items-center justify-center">
      <div className="max-w-lg w-full space-y-8">
        <div className="text-center">
          <h1 className="text-2xl font-semibold">Welcome to darkside</h1>
          <p className="mt-2 text-muted-foreground text-sm">
            Let's connect your GitHub account so darkside can listen for pushes
            and deploy your apps automatically.
          </p>
        </div>

        <div className="rounded-lg border p-6 space-y-4">
          <div className="flex items-center gap-3">
            <div className="rounded-full bg-primary text-primary-foreground w-7 h-7 flex items-center justify-center text-sm font-medium shrink-0">1</div>
            <div>
              <p className="font-medium text-sm">Create a GitHub App</p>
              <p className="text-xs text-muted-foreground mt-0.5">
                Darkside registers itself as a GitHub App so it can receive push webhooks
                and clone private repos. You'll be redirected to GitHub to confirm.
              </p>
            </div>
          </div>

          <label className="block">
            <span className="text-sm text-muted-foreground">
              Organization (optional — leave empty for your personal account)
            </span>
            <input
              value={org}
              onChange={(e) => setOrg(e.target.value)}
              placeholder="my-org"
              className="mt-1 w-full rounded-md border bg-background px-3 py-2 text-sm font-mono"
            />
          </label>

          <Button onClick={startGitHubAppFlow} className="w-full">
            Connect GitHub →
          </Button>
        </div>

        <div className="rounded-lg border border-dashed p-4 space-y-2 opacity-60">
          <div className="flex items-center gap-3">
            <div className="rounded-full border w-7 h-7 flex items-center justify-center text-sm font-medium shrink-0">2</div>
            <p className="text-sm">Sign in with GitHub — first user becomes admin</p>
          </div>
          <div className="flex items-center gap-3">
            <div className="rounded-full border w-7 h-7 flex items-center justify-center text-sm font-medium shrink-0">3</div>
            <p className="text-sm">Add your first app and push a commit</p>
          </div>
        </div>
      </div>
    </div>
  )
}

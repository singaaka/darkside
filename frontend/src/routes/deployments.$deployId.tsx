import { useEffect, useRef, useState } from "react"
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router"
import { useMutation, useQuery } from "@connectrpc/connect-query"
import { DeploymentService } from "@/gen/darkside/v1/api_pb"
import { deploymentsClient } from "@/lib/connect"
import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"

export const Route = createFileRoute("/deployments/$deployId")({
  component: DeploymentDetail,
})

const PHASES = ["build", "pre", "deploy", "post"] as const
type Phase = (typeof PHASES)[number]

function statusColour(status: string): string {
  switch (status) {
    case "succeeded":
    case "built":
      return "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400"
    case "failed":
      return "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400"
    default:
      return "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400"
  }
}

function DeploymentDetail() {
  const { deployId } = Route.useParams()
  const navigate = useNavigate()
  const detail = useQuery(
    DeploymentService.method.get,
    { id: deployId },
    {
      refetchInterval: (q) => {
        const s = q.state.data?.status
        return s === "succeeded" || s === "built" || s === "failed" ? false : 2000
      },
    },
  )
  const redeploy = useMutation(DeploymentService.method.redeploy, {
    onSuccess: (d) => navigate({ to: "/deployments/$deployId", params: { deployId: d.id } }),
  })
  const [phase, setPhase] = useState<Phase>("build")

  if (detail.isLoading) return <p className="text-sm text-muted-foreground">Loading…</p>
  if (detail.error) return <p className="text-sm text-destructive">{detail.error.message}</p>
  if (!detail.data) return null
  const d = detail.data

  return (
    <div className="space-y-6">
      <div>
        <Link to="/apps/$appId" params={{ appId: d.appId }} className="text-sm text-muted-foreground hover:underline">
          ← App
        </Link>
        <div className="mt-1 flex items-baseline gap-3 flex-wrap">
          <h1 className="text-2xl font-semibold">
            <code className="font-mono">{d.commitSha.slice(0, 7)}</code>
          </h1>
          <span className={cn("text-xs font-medium rounded-full px-2.5 py-0.5", statusColour(d.status))}>
            {d.status}
          </span>
          {d.environmentName ? (
            <span className="text-sm text-muted-foreground">→ {d.environmentName}</span>
          ) : null}
        </div>
        <p className="text-muted-foreground mt-1">{d.commitMessage}</p>
        {d.error ? (
          <pre className="mt-2 rounded-lg border border-destructive/50 bg-destructive/10 p-3 text-sm text-destructive whitespace-pre-wrap">
            {d.error}
          </pre>
        ) : null}
      </div>

      <div className="grid gap-4 md:grid-cols-3 text-sm">
        <Meta label="Image" value={d.imageTag || "—"} mono />
        <Meta label="Started" value={new Date(Number(d.startedAtUnix) * 1000).toLocaleString()} />
        <Meta
          label="Finished"
          value={
            d.finishedAtUnix
              ? new Date(Number(d.finishedAtUnix) * 1000).toLocaleString()
              : "—"
          }
        />
      </div>

      <div className="border-b">
        <nav className="-mb-px flex gap-6">
          {PHASES.map((p) => (
            <button
              key={p}
              type="button"
              onClick={() => setPhase(p)}
              className={cn(
                "py-3 text-sm border-b-2 capitalize transition-colors",
                phase === p
                  ? "border-foreground text-foreground"
                  : "border-transparent text-muted-foreground hover:text-foreground",
              )}
            >
              {p}
            </button>
          ))}
        </nav>
      </div>

      <LogStream deployId={deployId} phase={phase} />

      {d.nomadJobHcl ? (
        <details className="space-y-2">
          <summary className="cursor-pointer font-medium">Rendered nomad job</summary>
          <pre className="rounded-lg border bg-muted/40 p-3 text-xs overflow-x-auto font-mono">
            {d.nomadJobHcl}
          </pre>
        </details>
      ) : null}

      {d.envSnapshot ? (
        <details className="space-y-2">
          <summary className="cursor-pointer font-medium">Env snapshot (resolved at deploy time)</summary>
          <pre className="rounded-lg border bg-muted/40 p-3 text-xs overflow-x-auto font-mono">
            {d.envSnapshot}
          </pre>
        </details>
      ) : null}

      <div className="flex gap-3">
        <Button
          onClick={() => redeploy.mutate({ deploymentId: d.id })}
          disabled={redeploy.isPending || !d.environmentId}
          title={
            !d.environmentId
              ? "Redeploy requires a deployment with a resolved environment"
              : "Re-run the deploy pipeline for this commit + env"
          }
        >
          {redeploy.isPending ? "Queuing…" : "Redeploy this commit"}
        </Button>
        <Button asChild variant="outline">
          <Link to="/apps/$appId" params={{ appId: d.appId }}>
            Back to app
          </Link>
        </Button>
      </div>
      {redeploy.error ? (
        <p className="text-sm text-destructive">{redeploy.error.message}</p>
      ) : null}
    </div>
  )
}

function Meta({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="rounded-lg border p-3">
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className={cn("mt-1", mono && "font-mono text-sm")}>{value}</div>
    </div>
  )
}

function LogStream({ deployId, phase }: { deployId: string; phase: Phase }) {
  const [text, setText] = useState("")
  const [streaming, setStreaming] = useState(true)
  const preRef = useRef<HTMLPreElement | null>(null)

  useEffect(() => {
    setText("")
    setStreaming(true)
    const ac = new AbortController()
    ;(async () => {
      try {
        for await (const chunk of deploymentsClient.streamLogs(
          { deploymentId: deployId, phase },
          { signal: ac.signal },
        )) {
          setText((prev) => prev + chunk.chunk)
        }
      } catch {
        // Network error / aborted — finished marker shown via streaming=false.
      } finally {
        setStreaming(false)
      }
    })()
    return () => ac.abort()
  }, [deployId, phase])

  useEffect(() => {
    if (preRef.current) preRef.current.scrollTop = preRef.current.scrollHeight
  }, [text])

  return (
    <div className="rounded-lg border overflow-hidden">
      <div className="flex items-center justify-between px-3 py-1.5 border-b text-xs text-muted-foreground bg-muted/40">
        <span className="font-mono capitalize">{phase}</span>
        <span>{streaming ? "● live" : "● closed"}</span>
      </div>
      <pre
        ref={preRef}
        className="px-3 py-2 text-xs font-mono whitespace-pre overflow-auto max-h-[500px] min-h-[100px]"
      >
        {text || <span className="text-muted-foreground">No output yet.</span>}
      </pre>
    </div>
  )
}

import { useState, useEffect, useRef } from "react"
import { createFileRoute, Link } from "@tanstack/react-router"
import { useQuery as useRQQuery, useMutation as useRQMutation } from "@tanstack/react-query"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import { getDeploymentDetail, redeployFromJSON } from "@/lib/api"
import { deploymentsClient } from "@/lib/connect"
import type { DeploymentDetail } from "@/gen/darkside/v1/api_pb"

export const Route = createFileRoute("/deployments/$deployId")({
  component: DeploymentPage,
})

const LOG_PHASES = ["build", "pre", "deploy", "post"] as const
type Phase = (typeof LOG_PHASES)[number]
type LogTab = Phase | "nomad-job" | "build-job" | "pre-hook" | "post-hook" | "env"

function DeploymentPage() {
  const { deployId } = Route.useParams()
  const detail = useRQQuery({
    queryKey: ["deployment-detail", deployId],
    queryFn: () => getDeploymentDetail(deployId),
    refetchInterval: (q) => {
      const s = q.state.data?.status
      return s && ["succeeded", "failed", "built"].includes(s) ? false : 3000
    },
  })
  const [logTab, setLogTab] = useState<LogTab>("build")
  const redeployMutation = useRQMutation({
    mutationFn: () => redeployFromJSON(deployId),
    onSuccess: (d) => { window.location.href = `/deployments/${d.id}` },
  })

  if (detail.isLoading) return <p className="text-sm text-muted-foreground">Loading…</p>
  if (detail.error) return <p className="text-sm text-destructive">{(detail.error as Error).message}</p>
  const d = detail.data
  if (!d) return null

  const tabs: LogTab[] = ["build", "pre", "deploy", "post"]
  if (d.nomad_job_hcl) tabs.push("nomad-job")
  if (d.build_job_hcl) tabs.push("build-job")
  if (d.pre_hook_hcl) tabs.push("pre-hook")
  if (d.post_hook_hcl) tabs.push("post-hook")
  if (d.env_snapshot) tabs.push("env")

  return (
    <div className="space-y-6">
      <div>
        <Link to="/apps/$appId" params={{ appId: d.app_id }} className="text-sm text-muted-foreground hover:underline">
          ← App
        </Link>
        <div className="flex items-start justify-between mt-1">
          <div>
            <div className="flex items-center gap-3">
              <h1 className="text-xl font-semibold font-mono">{d.commit_sha.slice(0, 7)}</h1>
              <StatusBadge status={d.status} />
              <TriggerBadge type={d.trigger_type} />
              {d.trigger_branch && <code className="text-xs font-mono text-muted-foreground">{d.trigger_branch}</code>}
            </div>
            <p className="text-muted-foreground text-sm mt-0.5">{d.commit_message}</p>
            {d.image_tag && <code className="text-xs font-mono text-muted-foreground">{d.image_tag}</code>}
          </div>
          <Button variant="outline" size="sm" onClick={() => redeployMutation.mutate()} disabled={redeployMutation.isPending}>
            {redeployMutation.isPending ? "Queueing…" : "Redeploy"}
          </Button>
        </div>
        {d.error && (
          <div className="mt-3 rounded-lg border border-destructive/30 bg-destructive/5 p-3 text-sm text-destructive">
            {d.error}
          </div>
        )}
      </div>

      <div className="border-b">
        <nav className="-mb-px flex flex-wrap gap-4">
          {tabs.map((t) => (
            <button key={t} type="button" onClick={() => setLogTab(t)}
              className={cn("py-3 text-sm border-b-2 transition-colors whitespace-nowrap",
                logTab === t ? "border-foreground text-foreground" : "border-transparent text-muted-foreground hover:text-foreground")}>
              {tabLabel(t)}
            </button>
          ))}
        </nav>
      </div>

      {(LOG_PHASES as readonly string[]).includes(logTab)
        ? <LogStream deployId={deployId} phase={logTab as Phase} status={d.status} />
        : <CodeBlock content={tabContent(d, logTab)} />}
    </div>
  )
}

function tabLabel(t: LogTab): string {
  const m: Record<LogTab, string> = {
    build: "Build logs", pre: "Pre hook logs", deploy: "Deploy logs", post: "Post hook logs",
    "nomad-job": "Nomad job HCL", "build-job": "Build job HCL",
    "pre-hook": "Pre hook HCL", "post-hook": "Post hook HCL", env: "Env (decrypted)",
  }
  return m[t] ?? t
}

function tabContent(d: DeploymentDetail, t: LogTab): string {
  switch (t) {
    case "nomad-job": return d.nomad_job_hcl ?? ""
    case "build-job": return d.build_job_hcl ?? ""
    case "pre-hook": return d.pre_hook_hcl ?? ""
    case "post-hook": return d.post_hook_hcl ?? ""
    case "env": return d.env_snapshot ?? ""
    default: return ""
  }
}

function TriggerBadge({ type }: { type: string }) {
  const m: Record<string, string> = { github: "bg-slate-100 text-slate-600", manual: "bg-blue-100 text-blue-700", rollback: "bg-orange-100 text-orange-700" }
  return <span className={cn("text-xs px-1.5 py-0.5 rounded font-medium", m[type] ?? "bg-muted text-muted-foreground")}>{type}</span>
}

function StatusBadge({ status }: { status: string }) {
  const c = status === "succeeded" || status === "built" ? "bg-emerald-100 text-emerald-700" : status === "failed" ? "bg-red-100 text-red-700" : "bg-amber-100 text-amber-700"
  return <span className={cn("text-xs px-1.5 py-0.5 rounded font-medium", c)}>{status}</span>
}

function LogStream({ deployId, phase, status }: { deployId: string; phase: Phase; status: string }) {
  const [lines, setLines] = useState<string[]>([])
  const bottomRef = useRef<HTMLDivElement>(null)
  const boxRef = useRef<HTMLDivElement>(null)
  const [autoScroll, setAutoScroll] = useState(true)
  const isTerminal = ["succeeded", "built", "failed"].includes(status)

  useEffect(() => {
    setLines([])
    const ac = new AbortController()
    async function stream() {
      try {
        const s = await deploymentsClient.streamLogs({ deploymentId: deployId, phase })
        for await (const msg of s) {
          if (ac.signal.aborted) break
          setLines((prev) => [...prev, ...msg.chunk.split("\n").filter(Boolean)])
        }
      } catch { /* ended */ }
    }
    stream()
    return () => ac.abort()
  }, [deployId, phase])

  useEffect(() => {
    if (autoScroll) bottomRef.current?.scrollIntoView({ behavior: "smooth" })
  }, [lines, autoScroll])

  return (
    <div className="relative">
      <div ref={boxRef} onScroll={() => { const el = boxRef.current; if (el) setAutoScroll(el.scrollTop + el.clientHeight >= el.scrollHeight - 20) }}
        className="rounded-lg border bg-black text-green-400 font-mono text-xs p-4 h-96 overflow-y-auto">
        {lines.length === 0 && <span className="text-gray-600">{isTerminal ? "No output." : "Waiting…"}</span>}
        {lines.map((l, i) => <div key={i}>{l}</div>)}
        <div ref={bottomRef} />
      </div>
      {!autoScroll && (
        <button type="button" onClick={() => { setAutoScroll(true); bottomRef.current?.scrollIntoView() }}
          className="absolute bottom-3 right-3 rounded bg-background border px-2 py-1 text-xs hover:bg-muted">
          ↓ scroll to bottom
        </button>
      )}
    </div>
  )
}

function CodeBlock({ content }: { content: string }) {
  const [copied, setCopied] = useState(false)
  const copy = async () => { await navigator.clipboard.writeText(content); setCopied(true); setTimeout(() => setCopied(false), 1500) }
  return (
    <div className="rounded-lg border bg-muted/40 overflow-hidden">
      <div className="flex justify-end px-3 py-1.5 border-b text-xs text-muted-foreground">
        <button type="button" onClick={copy} className="hover:text-foreground">{copied ? "copied" : "copy"}</button>
      </div>
      <pre className="px-3 py-2 text-xs overflow-x-auto font-mono whitespace-pre-wrap max-h-96">
        {content || <span className="text-muted-foreground">No content.</span>}
      </pre>
    </div>
  )
}

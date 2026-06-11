import { useState } from "react"
import { createFileRoute, Link } from "@tanstack/react-router"
import { useMutation, useQuery } from "@connectrpc/connect-query"
import { useQuery as useRQQuery, useMutation as useRQMutation } from "@tanstack/react-query"
import { AppService, DeploymentService } from "@/gen/darkside/v1/api_pb"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import { getAppDetail, rotateKey, scaleApp, manualDeploy } from "@/lib/api"
import type { AppDetail } from "@/gen/darkside/v1/api_pb"

export const Route = createFileRoute("/apps/$appId")({
  component: AppDetail,
})

type Tab = "deployments" | "manifest" | "age-key" | "scale"

function AppDetail() {
  const { appId } = Route.useParams()
  const app = useQuery(AppService.method.get, { id: appId })
  const appDetail = useRQQuery({
    queryKey: ["app-detail", appId],
    queryFn: () => getAppDetail(appId),
  })
  const [tab, setTab] = useState<Tab>("deployments")

  if (app.isLoading) return <p className="text-sm text-muted-foreground">Loading…</p>
  if (app.error) return <p className="text-sm text-destructive">{app.error.message}</p>
  if (!app.data) return null

  const detail = appDetail.data

  return (
    <div className="space-y-6">
      <div>
        <Link to="/apps" className="text-sm text-muted-foreground hover:underline">← Apps</Link>
        <h1 className="text-2xl font-semibold mt-1">{app.data.name}</h1>
        <div className="flex items-center gap-4 mt-1 text-sm text-muted-foreground">
          <span className="font-mono">{app.data.repoFullName}</span>
          {detail && <span>branch: <code className="font-mono">{detail.branch}</code></span>}
        </div>
      </div>

      <div className="border-b">
        <nav className="-mb-px flex gap-6">
          {(["deployments", "manifest", "age-key", "scale"] as Tab[]).map((t) => (
            <button key={t} type="button" onClick={() => setTab(t)}
              className={cn("py-3 text-sm border-b-2 transition-colors",
                t === "age-key" ? "capitalize" : "capitalize",
                tab === t
                  ? "border-foreground text-foreground"
                  : "border-transparent text-muted-foreground hover:text-foreground",
              )}>
              {t === "age-key" ? "Age Key" : t.charAt(0).toUpperCase() + t.slice(1)}
            </button>
          ))}
        </nav>
      </div>

      {tab === "deployments" && <DeploymentsTab appId={appId} appDetail={detail ?? null} />}
      {tab === "manifest" && <ManifestTab appId={appId} />}
      {tab === "age-key" && <AgeKeyTab appId={appId} detail={detail ?? null} onUpdated={() => appDetail.refetch()} />}
      {tab === "scale" && <ScaleTab appId={appId} appName={app.data.name} />}
    </div>
  )
}

function statusColour(status: string): string {
  switch (status) {
    case "succeeded": case "built": return "text-emerald-600"
    case "failed": return "text-destructive"
    default: return "text-amber-600"
  }
}

function triggerBadge(type: string) {
  const map: Record<string, string> = {
    github: "bg-slate-100 text-slate-600",
    manual: "bg-blue-100 text-blue-700",
    rollback: "bg-orange-100 text-orange-700",
  }
  return (
    <span className={cn("text-xs px-1.5 py-0.5 rounded font-medium", map[type] ?? "bg-muted text-muted-foreground")}>
      {type}
    </span>
  )
}

function DeploymentsTab({ appId, appDetail }: { appId: string; appDetail: AppDetail | null }) {
  const [showDeploy, setShowDeploy] = useState(false)
  const [branch, setBranch] = useState("")
  const [sha, setSha] = useState("")
  const deployMutation = useRQMutation({
    mutationFn: () => manualDeploy(appId, branch || undefined, sha || undefined),
    onSuccess: () => { setShowDeploy(false); list.refetch() },
  })

  const list = useQuery(
    DeploymentService.method.list,
    { appId, limit: 50 },
    {
      refetchInterval: (q) => {
        const data = q.state.data?.deployments ?? []
        return data.some((d) => !["succeeded", "built", "failed"].includes(d.status)) ? 2000 : 10000
      },
    },
  )

  if (list.isLoading) return <p className="text-sm text-muted-foreground">Loading…</p>
  if (list.error) return <p className="text-sm text-destructive">{list.error.message}</p>

  const deployments = list.data?.deployments ?? []

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <p className="text-sm text-muted-foreground">
          {deployments.length === 0 ? "No deployments yet." : `${deployments.length} deployment(s)`}
        </p>
        <Button variant="outline" size="sm" onClick={() => setShowDeploy((s) => !s)}>
          {showDeploy ? "Cancel" : "Deploy manually"}
        </Button>
      </div>

      {showDeploy && (
        <div className="rounded-lg border p-4 space-y-3">
          <p className="text-sm font-medium">Manual deployment</p>
          <div className="grid grid-cols-2 gap-3">
            <label className="block text-sm">
              <span className="text-muted-foreground">Branch (optional)</span>
              <input value={branch} onChange={(e) => setBranch(e.target.value)}
                placeholder={appDetail?.branch ?? "main"}
                className="mt-1 w-full rounded-md border bg-background px-3 py-1.5 text-sm font-mono" />
            </label>
            <label className="block text-sm">
              <span className="text-muted-foreground">Commit SHA (optional)</span>
              <input value={sha} onChange={(e) => setSha(e.target.value)}
                placeholder="HEAD"
                className="mt-1 w-full rounded-md border bg-background px-3 py-1.5 text-sm font-mono" />
            </label>
          </div>
          {deployMutation.error && <p className="text-sm text-destructive">{deployMutation.error.message}</p>}
          <Button size="sm" onClick={() => deployMutation.mutate()} disabled={deployMutation.isPending}>
            {deployMutation.isPending ? "Queueing…" : "Trigger deploy"}
          </Button>
        </div>
      )}

      {deployments.length === 0 ? (
        <div className="rounded-lg border border-dashed p-8 text-center text-sm text-muted-foreground">
          Push a commit to <code className="font-mono">{appDetail?.branch ?? "main"}</code> to trigger a deploy.
        </div>
      ) : (
        <ul className="divide-y rounded-lg border">
          {deployments.map((d) => (
            <li key={d.id} className="p-4 flex items-center justify-between gap-4">
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2 text-sm">
                  <code className="font-mono text-xs bg-muted px-1.5 py-0.5 rounded">{d.commitSha.slice(0, 7)}</code>
                  <span className="truncate">{d.commitMessage || "(no message)"}</span>
                  {triggerBadge("github")}
                </div>
                <div className="mt-1 text-xs text-muted-foreground flex gap-3">
                  <span className={statusColour(d.status)}>{d.status}</span>
                  {d.imageTag ? <span className="font-mono">{d.imageTag}</span> : null}
                  <span>{new Date(Number(d.startedAtUnix) * 1000).toLocaleString()}</span>
                </div>
              </div>
              <Button asChild variant="outline" size="sm">
                <Link to="/deployments/$deployId" params={{ deployId: d.id }}>Logs</Link>
              </Button>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

function ManifestTab({ appId }: { appId: string }) {
  const sample = useQuery(AppService.method.getManifestSample, { appId })
  if (sample.isLoading) return <p className="text-sm text-muted-foreground">Loading…</p>
  if (sample.error) return <p className="text-sm text-destructive">{sample.error.message}</p>
  return (
    <div className="space-y-4">
      <p className="text-sm text-muted-foreground">
        Paste this as <code className="font-mono">darkside.toml</code> at your repo root.
        Pushes to the configured <code className="font-mono">branch</code> trigger builds automatically.
      </p>
      <CodeBlock content={sample.data?.toml ?? ""} filename="darkside.toml" />
    </div>
  )
}

function AgeKeyTab({ appId, detail, onUpdated }: { appId: string; detail: AppDetail | null; onUpdated: () => void }) {
  const [showPrivate, setShowPrivate] = useState(false)
  const [rotated, setRotated] = useState<{ age_private_key: string; age_public_key: string; age_key_id: string; setup_instructions: string; toml_snippet: string } | null>(null)
  const rotateMutation = useRQMutation({
    mutationFn: () => rotateKey(appId),
    onSuccess: (data) => { setRotated(data); onUpdated() },
  })

  if (!detail) return <p className="text-sm text-muted-foreground">Loading…</p>

  return (
    <div className="space-y-6">
      <div className="rounded-lg border p-5 space-y-4">
        <div className="flex items-baseline justify-between">
          <h3 className="font-medium">Current key</h3>
          <code className="text-xs font-mono text-muted-foreground">{detail.age_key_id}</code>
        </div>
        <div>
          <p className="text-xs text-muted-foreground mb-1">Public key (commit to repo, used for encryption)</p>
          <CodeBlock content={detail.age_public_key} />
        </div>
        {!rotated && (
          <div>
            <p className="text-xs text-muted-foreground mb-1">
              Private key (stored in darkside DB — shown here for full visibility)
            </p>
            <div className="rounded-lg border bg-muted/40 overflow-hidden">
              <div className="flex items-center justify-between px-3 py-1.5 border-b text-xs text-muted-foreground">
                <span>private key</span>
                <button type="button" onClick={() => setShowPrivate((s) => !s)} className="hover:text-foreground">
                  {showPrivate ? "hide" : "reveal"}
                </button>
              </div>
              <pre className="px-3 py-2 text-xs font-mono">
                {showPrivate ? "AGE-SECRET-KEY-... (fetch via /api/v1/apps/" + appId + ")" : "•".repeat(48)}
              </pre>
            </div>
          </div>
        )}
      </div>

      {rotated ? (
        <div className="rounded-lg border border-green-200 bg-green-50 p-5 space-y-4">
          <p className="font-medium text-green-800 text-sm">Key rotated to {rotated.age_key_id}</p>
          <p className="text-xs text-green-700">
            Save the new private key now — it won't be shown again after you leave this page.
          </p>
          <CodeBlock content={rotated.age_private_key} filename={`${rotated.age_key_id}-private.txt`} />
          <CodeBlock content={rotated.setup_instructions} filename="rotate-key.sh" />
          <CodeBlock content={rotated.toml_snippet} filename="darkside.toml snippet" />
          <Button variant="outline" size="sm" onClick={() => setRotated(null)}>Done</Button>
        </div>
      ) : (
        <div className="space-y-3">
          <p className="text-sm text-muted-foreground">
            Rotating the key generates a new keypair. You'll need to re-encrypt your <code className="font-mono">env.age</code>{" "}
            file with the new public key and update <code className="font-mono">key_id</code> in <code className="font-mono">darkside.toml</code>.
          </p>
          {rotateMutation.error && <p className="text-sm text-destructive">{rotateMutation.error.message}</p>}
          <Button variant="destructive" size="sm" onClick={() => rotateMutation.mutate()} disabled={rotateMutation.isPending}>
            {rotateMutation.isPending ? "Rotating…" : "Rotate key"}
          </Button>
        </div>
      )}
    </div>
  )
}

function ScaleTab({ appId, appName }: { appId: string; appName: string }) {
  const [count, setCount] = useState(1)
  const [result, setResult] = useState<{ warning: string } | null>(null)
  const scaleMutation = useRQMutation({
    mutationFn: () => scaleApp(appId, count),
    onSuccess: (data) => setResult(data),
  })

  return (
    <div className="space-y-4 max-w-sm">
      <p className="text-sm text-muted-foreground">
        Scale the number of running instances of <strong>{appName}</strong> immediately, without
        triggering a full rebuild.
      </p>
      <label className="block text-sm">
        <span className="font-medium">Instance count</span>
        <input type="number" min={1} max={20} value={count} onChange={(e) => setCount(Number(e.target.value))}
          className="mt-1 w-24 rounded-md border bg-background px-3 py-1.5 text-sm" />
      </label>
      {scaleMutation.error && <p className="text-sm text-destructive">{scaleMutation.error.message}</p>}
      {result && (
        <div className="rounded-lg border border-yellow-200 bg-yellow-50 p-3 text-xs text-yellow-800">
          ⚠ {result.warning}
        </div>
      )}
      <Button size="sm" onClick={() => scaleMutation.mutate()} disabled={scaleMutation.isPending}>
        {scaleMutation.isPending ? "Scaling…" : `Scale to ${count}`}
      </Button>
    </div>
  )
}

function CodeBlock({ content, filename }: { content: string; filename?: string }) {
  const [copied, setCopied] = useState(false)
  const copy = async () => {
    await navigator.clipboard.writeText(content)
    setCopied(true)
    setTimeout(() => setCopied(false), 1500)
  }
  return (
    <div className="rounded-lg border bg-muted/40 overflow-hidden mt-2">
      {filename && (
        <div className="flex items-center justify-between px-3 py-1.5 border-b text-xs text-muted-foreground">
          <span className="font-mono">{filename}</span>
          <button type="button" onClick={copy} className="hover:text-foreground">{copied ? "copied" : "copy"}</button>
        </div>
      )}
      <pre className="px-3 py-2 text-xs overflow-x-auto font-mono whitespace-pre-wrap">{content}</pre>
    </div>
  )
}

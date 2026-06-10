import { useState } from "react"
import { createFileRoute, Link } from "@tanstack/react-router"
import { useMutation, useQuery } from "@connectrpc/connect-query"
import { AppService, DeploymentService, EnvironmentService } from "@/gen/darkside/v1/api_pb"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

export const Route = createFileRoute("/apps/$appId")({
  component: AppDetail,
})

type Tab = "deployments" | "manifest" | "environments"

function AppDetail() {
  const { appId } = Route.useParams()
  const app = useQuery(AppService.method.get, { id: appId })
  const [tab, setTab] = useState<Tab>("deployments")

  if (app.isLoading) return <p className="text-sm text-muted-foreground">Loading…</p>
  if (app.error) return <p className="text-sm text-destructive">{app.error.message}</p>
  if (!app.data) return null

  return (
    <div className="space-y-6">
      <div>
        <Link to="/apps" className="text-sm text-muted-foreground hover:underline">
          ← Apps
        </Link>
        <h1 className="text-2xl font-semibold mt-1">{app.data.name}</h1>
        <p className="text-muted-foreground font-mono text-sm">{app.data.repoFullName}</p>
      </div>

      <div className="border-b">
        <nav className="-mb-px flex gap-6">
          {(["deployments", "manifest", "environments"] as Tab[]).map((t) => (
            <button
              key={t}
              type="button"
              onClick={() => setTab(t)}
              className={cn(
                "py-3 text-sm border-b-2 capitalize transition-colors",
                tab === t
                  ? "border-foreground text-foreground"
                  : "border-transparent text-muted-foreground hover:text-foreground",
              )}
            >
              {t}
            </button>
          ))}
        </nav>
      </div>

      {tab === "deployments" && <DeploymentsTab appId={appId} />}
      {tab === "manifest" && <ManifestTab appId={appId} />}
      {tab === "environments" && <EnvironmentsTab appId={appId} appName={app.data.name} />}
    </div>
  )
}

function statusColour(status: string): string {
  switch (status) {
    case "succeeded":
    case "built":
      return "text-emerald-600"
    case "failed":
      return "text-destructive"
    case "pending":
    case "cloning":
    case "building":
    case "pre":
    case "deploying":
    case "post":
      return "text-amber-600"
    default:
      return "text-muted-foreground"
  }
}

function DeploymentsTab({ appId }: { appId: string }) {
  // Poll the list so in-flight builds visibly tick along. 2s feels live; back
  // off once nothing is in-flight.
  const list = useQuery(
    DeploymentService.method.list,
    { appId, limit: 50 },
    {
      refetchInterval: (q) => {
        const data = q.state.data?.deployments ?? []
        const anyRunning = data.some(
          (d) => !["succeeded", "built", "failed"].includes(d.status),
        )
        return anyRunning ? 2000 : 10000
      },
    },
  )

  if (list.isLoading) return <p className="text-sm text-muted-foreground">Loading…</p>
  if (list.error) return <p className="text-sm text-destructive">{list.error.message}</p>

  const deployments = list.data?.deployments ?? []
  if (deployments.length === 0) {
    return (
      <div className="rounded-lg border border-dashed p-8 text-center text-sm text-muted-foreground">
        No deployments yet. Push a commit to the branch mapped in <code>darkside.toml</code>.
      </div>
    )
  }

  return (
    <ul className="divide-y rounded-lg border">
      {deployments.map((d) => (
        <li key={d.id} className="p-4 flex items-center justify-between gap-4">
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2 text-sm">
              <code className="font-mono text-xs bg-muted px-1.5 py-0.5 rounded">
                {d.commitSha.slice(0, 7)}
              </code>
              <span className="truncate">{d.commitMessage || "(no message)"}</span>
            </div>
            <div className="mt-1 text-xs text-muted-foreground flex gap-3">
              <span className={statusColour(d.status)}>{d.status}</span>
              {d.environmentName ? <span>→ {d.environmentName}</span> : null}
              {d.imageTag ? <span className="font-mono">{d.imageTag}</span> : null}
              <span>{new Date(Number(d.startedAtUnix) * 1000).toLocaleString()}</span>
            </div>
          </div>
          <Button asChild variant="outline" size="sm">
            <Link to="/deployments/$deployId" params={{ deployId: d.id }}>
              Logs
            </Link>
          </Button>
        </li>
      ))}
    </ul>
  )
}

function ManifestTab({ appId }: { appId: string }) {
  const sample = useQuery(AppService.method.getManifestSample, { appId })
  if (sample.isLoading) return <p className="text-sm text-muted-foreground">Loading…</p>
  if (sample.error) return <p className="text-sm text-destructive">{sample.error.message}</p>
  return (
    <div className="space-y-4">
      <p className="text-sm text-muted-foreground">
        Paste this as <code className="font-mono">darkside.toml</code> at the root of your repo,
        then edit comments to fit your app. Each push to the configured branch triggers a build +
        deploy.
      </p>
      <CodeBlock content={sample.data?.toml ?? ""} filename="darkside.toml" />
    </div>
  )
}

function EnvironmentsTab({ appId, appName }: { appId: string; appName: string }) {
  const envs = useQuery(EnvironmentService.method.list, { appId })
  const [showCreate, setShowCreate] = useState(false)
  if (envs.isLoading) return <p className="text-sm text-muted-foreground">Loading…</p>
  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <p className="text-sm text-muted-foreground">
          Each environment has its own age keypair. The private key is stored in darkside&apos;s DB
          (unredacted) and shown to you so you can copy it locally. The public key (recipient) is
          committed to your repo so anyone can encrypt new env files.
        </p>
        <Button onClick={() => setShowCreate((s) => !s)}>
          {showCreate ? "Cancel" : "New environment"}
        </Button>
      </div>
      {showCreate && (
        <CreateEnvForm
          appId={appId}
          onDone={() => {
            setShowCreate(false)
            envs.refetch()
          }}
        />
      )}
      {(envs.data?.environments.length ?? 0) === 0 ? (
        <div className="rounded-lg border border-dashed p-8 text-center text-sm text-muted-foreground">
          No environments yet. Create one to generate an age keypair.
        </div>
      ) : (
        <div className="space-y-4">
          {envs.data!.environments.map((e) => (
            <EnvironmentCard key={e.id} appId={appId} appName={appName} envName={e.name} />
          ))}
        </div>
      )}
    </div>
  )
}

function CreateEnvForm({ appId, onDone }: { appId: string; onDone: () => void }) {
  const [name, setName] = useState("")
  const create = useMutation(EnvironmentService.method.create, { onSuccess: onDone })
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault()
        create.mutate({ appId, name: name.trim() })
      }}
      className="space-y-3 rounded-lg border p-4"
    >
      <label className="block text-sm">
        <span className="block font-medium">Environment name</span>
        <span className="block text-xs text-muted-foreground mt-0.5">
          Lowercase slug, e.g. <code>production</code> or <code>staging</code>. Used in the
          encrypted env file name and the nomad job name.
        </span>
        <input
          value={name}
          onChange={(e) => setName(e.target.value)}
          required
          pattern="[a-z][a-z0-9-]{0,19}"
          placeholder="production"
          className="mt-2 w-full max-w-sm rounded-md border bg-background px-3 py-2 text-sm font-mono"
        />
      </label>
      {create.error ? <p className="text-sm text-destructive">{create.error.message}</p> : null}
      <Button type="submit" disabled={create.isPending}>
        {create.isPending ? "Generating keypair…" : "Create environment"}
      </Button>
    </form>
  )
}

function EnvironmentCard({
  appId,
  envName,
}: {
  appId: string
  appName: string
  envName: string
}) {
  const detail = useQuery(EnvironmentService.method.get, { appId, name: envName })
  const [showPrivate, setShowPrivate] = useState(false)
  if (detail.isLoading) return <div className="rounded-lg border p-4 text-sm">Loading…</div>
  if (detail.error)
    return <div className="rounded-lg border p-4 text-sm text-destructive">{detail.error.message}</div>
  if (!detail.data) return null
  const { environment: env, setup } = detail.data
  if (!env || !setup) return null
  return (
    <div className="rounded-lg border p-5 space-y-4">
      <div className="flex items-baseline justify-between">
        <h3 className="font-medium">{env.name}</h3>
        <div className="text-xs text-muted-foreground font-mono">
          recipient: {env.publicKey.slice(0, 16)}…
        </div>
      </div>
      <details>
        <summary className="cursor-pointer text-sm font-medium">Setup commands</summary>
        <CodeBlock content={setup.bashCommands} filename="setup.sh" />
      </details>
      <details>
        <summary className="cursor-pointer text-sm font-medium">Add to darkside.toml</summary>
        <CodeBlock content={setup.manifestSnippet} filename="darkside.toml" />
      </details>
      <details>
        <summary className="cursor-pointer text-sm font-medium">Keys</summary>
        <div className="mt-2 space-y-2 text-sm">
          <div>
            <div className="text-muted-foreground text-xs">
              Public key (recipient — commit at {setup.recipientPath})
            </div>
            <code className="block mt-1 break-all font-mono text-xs bg-muted p-2 rounded">
              {env.publicKey}
            </code>
          </div>
          <div>
            <div className="flex items-center justify-between">
              <div className="text-muted-foreground text-xs">Private key (keep local)</div>
              <button
                type="button"
                className="text-xs underline"
                onClick={() => setShowPrivate((s) => !s)}
              >
                {showPrivate ? "hide" : "reveal"}
              </button>
            </div>
            <code className="block mt-1 break-all font-mono text-xs bg-muted p-2 rounded">
              {showPrivate ? env.privateKey : "•".repeat(64)}
            </code>
          </div>
        </div>
      </details>
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
      <div className="flex items-center justify-between px-3 py-1.5 border-b text-xs text-muted-foreground">
        <span className="font-mono">{filename ?? ""}</span>
        <button type="button" onClick={copy} className="hover:text-foreground">
          {copied ? "copied" : "copy"}
        </button>
      </div>
      <pre className="px-3 py-2 text-xs overflow-x-auto font-mono whitespace-pre-wrap">{content}</pre>
    </div>
  )
}

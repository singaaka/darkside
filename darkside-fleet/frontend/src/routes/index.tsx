import { useState } from "react"
import { createFileRoute } from "@tanstack/react-router"
import { useMutation, useQuery } from "@connectrpc/connect-query"
import { NodeService, ClusterConfigService } from "@/gen/fleet/v1/api_pb"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

export const Route = createFileRoute("/")({
  component: NodesPage,
})

function NodesPage() {
  const nodes = useQuery(NodeService.method.list)
  const [showAdd, setShowAdd] = useState(false)

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold">Cluster Nodes</h1>
          <p className="text-muted-foreground text-sm mt-1">
            Add nodes to provision them with Docker, Nomad, and Consul.
            The first node added automatically hosts darkside.
          </p>
        </div>
        <Button onClick={() => setShowAdd((s) => !s)}>
          {showAdd ? "Cancel" : "+ Add node"}
        </Button>
      </div>

      {showAdd && <AddNodeForm onDone={() => { setShowAdd(false); nodes.refetch() }} />}

      {nodes.isLoading ? (
        <p className="text-sm text-muted-foreground">Loading…</p>
      ) : nodes.error ? (
        <p className="text-sm text-destructive">{nodes.error.message}</p>
      ) : (nodes.data?.nodes.length ?? 0) === 0 ? (
        <div className="rounded-lg border border-dashed p-12 text-center">
          <p className="text-muted-foreground text-sm">No nodes yet.</p>
          <p className="text-xs text-muted-foreground mt-1">
            Add your first node to start the cluster. It will automatically be designated
            as the darkside host node.
          </p>
        </div>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {nodes.data!.nodes.map((n) => (
            <NodeCard key={n.id} node={n} onUpdated={() => nodes.refetch()} />
          ))}
        </div>
      )}
    </div>
  )
}

function statusColour(status: string): string {
  switch (status) {
    case "active": return "bg-emerald-100 text-emerald-700"
    case "provisioning": return "bg-amber-100 text-amber-700"
    case "draining": case "removed": return "bg-red-100 text-red-700"
    default: return "bg-muted text-muted-foreground"
  }
}

function NodeCard({ node, onUpdated }: { node: any; onUpdated: () => void }) {
  const [showConfig, setShowConfig] = useState(false)
  const config = useQuery(ClusterConfigService.method.getNodeConfig, { nodeId: node.id }, { enabled: showConfig })
  const setPaas = useMutation(NodeService.method.setPaasNode, { onSuccess: onUpdated })
  const del = useMutation(NodeService.method.delete, { onSuccess: onUpdated })

  return (
    <div className={cn("rounded-lg border p-5 space-y-3", node.isDarksidePaasNode && "border-primary ring-1 ring-primary")}>
      <div className="flex items-start justify-between">
        <div>
          <div className="flex items-center gap-2">
            <span className="font-medium">{node.name}</span>
            {node.isDarksidePaasNode && (
              <span className="text-xs bg-primary text-primary-foreground px-1.5 py-0.5 rounded">darkside host</span>
            )}
          </div>
          <p className="font-mono text-xs text-muted-foreground mt-0.5">{node.publicIp}</p>
        </div>
        <span className={cn("text-xs px-1.5 py-0.5 rounded", statusColour(node.status))}>{node.status}</span>
      </div>

      {node.tags.length > 0 && (
        <div className="flex flex-wrap gap-1">
          {node.tags.map((t: string) => (
            <span key={t} className="text-xs bg-muted px-1.5 py-0.5 rounded">{t}</span>
          ))}
        </div>
      )}

      <div className="flex flex-wrap gap-2 pt-1">
        <Button variant="outline" size="sm" onClick={() => setShowConfig((s) => !s)}>
          {showConfig ? "Hide config" : "View config"}
        </Button>
        {!node.isDarksidePaasNode && (
          <Button variant="outline" size="sm" onClick={() => setPaas.mutate({ nodeId: node.id })} disabled={setPaas.isPending}>
            Set as darkside host
          </Button>
        )}
        <Button variant="destructive" size="sm" onClick={() => del.mutate({ id: node.id })} disabled={del.isPending}>
          Delete
        </Button>
      </div>

      {showConfig && (
        <div className="space-y-2 pt-2 border-t">
          {config.isLoading ? <p className="text-xs text-muted-foreground">Loading configs…</p> : null}
          {config.data && (
            <ConfigTabs config={config.data} />
          )}
        </div>
      )}
    </div>
  )
}

function ConfigTabs({ config }: { config: any }) {
  const [tab, setTab] = useState<"nomad" | "consul" | "traefik">("nomad")
  const tabs = [
    { key: "nomad" as const, label: "nomad.hcl", content: config.nomadHcl },
    { key: "consul" as const, label: "consul.hcl", content: config.consulHcl },
    { key: "traefik" as const, label: "traefik.yml", content: config.traefikYml },
  ]
  return (
    <div className="rounded border overflow-hidden text-xs">
      <div className="flex border-b bg-muted/30">
        {tabs.map((t) => (
          <button key={t.key} type="button" onClick={() => setTab(t.key)}
            className={cn("px-3 py-1.5 font-mono transition-colors", tab === t.key ? "bg-background" : "text-muted-foreground hover:text-foreground")}>
            {t.label}
          </button>
        ))}
      </div>
      <pre className="p-3 overflow-x-auto max-h-48 font-mono whitespace-pre-wrap">
        {tabs.find((t) => t.key === tab)?.content || "(empty)"}
      </pre>
    </div>
  )
}

function AddNodeForm({ onDone }: { onDone: () => void }) {
  const [form, setForm] = useState({ name: "", public_ip: "", ssh_user: "ubuntu", ssh_key_path: "~/.ssh/id_rsa", tags: "" })
  const add = useMutation(NodeService.method.add, { onSuccess: onDone })

  const submit = (e: React.FormEvent) => {
    e.preventDefault()
    add.mutate({
      name: form.name,
      publicIp: form.public_ip,
      sshUser: form.ssh_user,
      sshKeyPath: form.ssh_key_path,
      tags: form.tags.split(",").map((t) => t.trim()).filter(Boolean),
    })
  }

  const f = (k: keyof typeof form) => (e: React.ChangeEvent<HTMLInputElement>) =>
    setForm((s) => ({ ...s, [k]: e.target.value }))

  return (
    <form onSubmit={submit} className="rounded-lg border p-5 space-y-4">
      <h2 className="font-medium text-sm">Add node</h2>
      <div className="grid gap-3 sm:grid-cols-2">
        <Field label="Name (optional)"><input value={form.name} onChange={f("name")} placeholder="web-1" className={inputCls} /></Field>
        <Field label="Public IP *"><input value={form.public_ip} onChange={f("public_ip")} placeholder="1.2.3.4" required className={inputCls} /></Field>
        <Field label="SSH user"><input value={form.ssh_user} onChange={f("ssh_user")} placeholder="ubuntu" className={inputCls} /></Field>
        <Field label="SSH key path"><input value={form.ssh_key_path} onChange={f("ssh_key_path")} placeholder="~/.ssh/id_rsa" className={inputCls} /></Field>
        <Field label="Tags (comma separated)" hint="e.g. web,db">
          <input value={form.tags} onChange={f("tags")} placeholder="web,db" className={inputCls} />
        </Field>
      </div>
      {add.error && <p className="text-sm text-destructive">{add.error.message}</p>}
      <Button type="submit" disabled={add.isPending}>{add.isPending ? "Queueing provision…" : "Add node"}</Button>
    </form>
  )
}

const inputCls = "w-full rounded-md border bg-background px-3 py-1.5 text-sm font-mono"

function Field({ label, hint, children }: { label: string; hint?: string; children: React.ReactNode }) {
  return (
    <label className="block">
      <span className="block text-xs font-medium text-muted-foreground mb-1">{label}</span>
      {hint && <span className="block text-xs text-muted-foreground mb-1">{hint}</span>}
      {children}
    </label>
  )
}

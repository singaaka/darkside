import { useState, useEffect } from "react"
import { createFileRoute } from "@tanstack/react-router"
import { useMutation, useQuery } from "@connectrpc/connect-query"
import { SettingsService } from "@/gen/nm/v1/api_pb"
import { Button } from "@/components/ui/button"

export const Route = createFileRoute("/settings")({
  component: SettingsPage,
})

function SettingsPage() {
  const settings = useQuery(SettingsService.method.get)
  const update = useMutation(SettingsService.method.update, { onSuccess: () => settings.refetch() })

  const [domain, setDomain] = useState("")
  const [registryPort, setRegistryPort] = useState(5000)
  const [saved, setSaved] = useState(false)

  useEffect(() => {
    if (settings.data) {
      setDomain(settings.data.domain)
      setRegistryPort(settings.data.registryPort || 5000)
    }
  }, [settings.data])

  const save = (e: React.FormEvent) => {
    e.preventDefault()
    update.mutate({ domain, registryPort }, { onSuccess: () => { setSaved(true); setTimeout(() => setSaved(false), 2000) } })
  }

  return (
    <div className="space-y-6 max-w-lg">
      <h1 className="text-2xl font-semibold">Settings</h1>

      <form onSubmit={save} className="rounded-lg border p-5 space-y-4">
        <div>
          <label className="block text-sm font-medium">Domain</label>
          <p className="text-xs text-muted-foreground mt-0.5">
            The domain where darkside-paas will be accessible (e.g. <code>darkside.example.com</code>).
            Used when generating Traefik routing rules.
          </p>
          <input value={domain} onChange={(e) => setDomain(e.target.value)} placeholder="darkside.example.com"
            className="mt-2 w-full rounded-md border bg-background px-3 py-2 text-sm font-mono" />
        </div>

        <div>
          <label className="block text-sm font-medium">Private registry port</label>
          <p className="text-xs text-muted-foreground mt-0.5">
            Port for the private Docker registry deployed on the paas node.
          </p>
          <input type="number" value={registryPort} onChange={(e) => setRegistryPort(Number(e.target.value))} min={1024} max={65535}
            className="mt-2 w-32 rounded-md border bg-background px-3 py-2 text-sm font-mono" />
        </div>

        {update.error && <p className="text-sm text-destructive">{update.error.message}</p>}
        <Button type="submit" disabled={update.isPending}>
          {saved ? "Saved ✓" : update.isPending ? "Saving…" : "Save settings"}
        </Button>
      </form>

      {settings.data?.darksidePaasNodeId && (
        <div className="rounded-lg border p-4 space-y-1">
          <p className="text-sm font-medium">darkside-paas node</p>
          <p className="text-xs font-mono text-muted-foreground">{settings.data.darksidePaasNodeId}</p>
        </div>
      )}

      <div className="rounded-lg border bg-muted/30 p-4 space-y-2 text-xs text-muted-foreground">
        <p className="font-medium text-foreground">How darkside node manager works</p>
        <ol className="list-decimal list-inside space-y-1">
          <li>Set your domain above.</li>
          <li>Add your first node (EC2 instance or any Linux VPS with SSH access).</li>
          <li>Node manager runs ansible to install Docker, Nomad, Consul, and CNI.</li>
          <li>The first node becomes the darkside-paas node — it hosts the private image registry and the darkside-paas app.</li>
          <li>Add more nodes to grow the cluster. Nomad schedules your apps across all nodes.</li>
          <li>Point your domain's DNS at your nodes and configure your load balancer (or use round-robin DNS).</li>
        </ol>
      </div>
    </div>
  )
}

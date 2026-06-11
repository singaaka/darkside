import { useState } from "react"
import { createFileRoute } from "@tanstack/react-router"
import { useQuery } from "@connectrpc/connect-query"
import { PlaybookService } from "@/gen/fleet/v1/api_pb"
import { cn } from "@/lib/utils"

export const Route = createFileRoute("/playbooks")({
  component: PlaybooksPage,
})

function PlaybooksPage() {
  const list = useQuery(PlaybookService.method.list)
  const [selected, setSelected] = useState<string | null>(null)

  const playbooks = list.data?.playbooks ?? []

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">Ansible Playbooks</h1>
        <p className="text-sm text-muted-foreground mt-1">
          The exact YAML the runner executes on your nodes. Audit before, copy after.
        </p>
      </div>

      {list.isLoading ? (
        <p className="text-sm text-muted-foreground">Loading…</p>
      ) : list.error ? (
        <p className="text-sm text-destructive">{list.error.message}</p>
      ) : playbooks.length === 0 ? (
        <div className="rounded-lg border border-dashed p-8 text-center text-sm text-muted-foreground">
          No playbooks found in the playbook directory.
        </div>
      ) : (
        <div className="grid gap-4 md:grid-cols-[280px_1fr]">
          <ul className="rounded-lg border divide-y">
            {playbooks.map((p) => (
              <li key={p.name}>
                <button
                  type="button"
                  onClick={() => setSelected(p.name)}
                  className={cn(
                    "w-full text-left px-4 py-3 hover:bg-muted/40 transition-colors",
                    selected === p.name && "bg-muted/60",
                  )}
                >
                  <div className="font-mono text-sm">{p.name}</div>
                  {p.description && (
                    <div className="text-xs text-muted-foreground mt-0.5 line-clamp-2">
                      {p.description}
                    </div>
                  )}
                  <div className="text-[10px] text-muted-foreground mt-1">
                    {p.sizeBytes.toLocaleString()} bytes
                  </div>
                </button>
              </li>
            ))}
          </ul>

          <div>
            {selected ? (
              <PlaybookViewer name={selected} />
            ) : (
              <div className="rounded-lg border border-dashed p-8 text-center text-sm text-muted-foreground">
                Pick a playbook on the left.
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  )
}

function PlaybookViewer({ name }: { name: string }) {
  const pb = useQuery(PlaybookService.method.get, { name })
  const [copied, setCopied] = useState(false)

  if (pb.isLoading) return <p className="text-sm text-muted-foreground">Loading…</p>
  if (pb.error) return <p className="text-sm text-destructive">{pb.error.message}</p>
  if (!pb.data) return null

  const copy = async () => {
    await navigator.clipboard.writeText(pb.data!.content)
    setCopied(true)
    setTimeout(() => setCopied(false), 1500)
  }

  return (
    <div className="rounded-lg border overflow-hidden">
      <div className="flex items-center justify-between px-3 py-2 border-b text-xs bg-muted/30">
        <span className="font-mono">{pb.data.name}</span>
        <button type="button" onClick={copy} className="text-muted-foreground hover:text-foreground">
          {copied ? "copied" : "copy"}
        </button>
      </div>
      <pre className="px-4 py-3 text-xs overflow-x-auto font-mono whitespace-pre max-h-[600px] overflow-y-auto">
        {pb.data.content}
      </pre>
    </div>
  )
}

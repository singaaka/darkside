import { useState, useEffect, useRef } from "react"
import { createFileRoute } from "@tanstack/react-router"
import { useQuery } from "@connectrpc/connect-query"
import { JobService } from "@/gen/nm/v1/api_pb"
import { cn } from "@/lib/utils"

export const Route = createFileRoute("/jobs")({
  component: JobsPage,
})

function JobsPage() {
  const jobs = useQuery(
    JobService.method.list,
    { limit: 50 },
    {
      refetchInterval: (q) => {
        const data = q.state.data?.jobs ?? []
        return data.some((j) => j.status === "pending" || j.status === "running") ? 2000 : 10000
      },
    },
  )
  const [selected, setSelected] = useState<string | null>(null)

  const list = jobs.data?.jobs ?? []

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">Jobs</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Async operations (add node, delete node, migrate paas). Darkside shows you everything — no hidden state.
        </p>
      </div>

      {list.length === 0 ? (
        <div className="rounded-lg border border-dashed p-8 text-center text-sm text-muted-foreground">
          No jobs yet. Add a node to see provisioning jobs here.
        </div>
      ) : (
        <div className="space-y-2">
          {list.map((j) => (
            <div key={j.id} className={cn("rounded-lg border overflow-hidden", selected === j.id && "ring-1 ring-primary")}>
              <button type="button" onClick={() => setSelected(selected === j.id ? null : j.id)}
                className="w-full p-4 flex items-center justify-between text-left hover:bg-muted/30 transition-colors">
                <div>
                  <div className="flex items-center gap-2 text-sm">
                    <span className="font-medium font-mono">{j.type}</span>
                    <JobStatusBadge status={j.status} />
                  </div>
                  <p className="text-xs text-muted-foreground mt-0.5">
                    {new Date(Number(j.createdAtUnix) * 1000).toLocaleString()}
                    {j.startedAtUnix > 0 && ` → started ${new Date(Number(j.startedAtUnix) * 1000).toLocaleTimeString()}`}
                  </p>
                </div>
                <span className="text-muted-foreground text-xs">{selected === j.id ? "▲" : "▼"}</span>
              </button>
              {selected === j.id && (
                <div className="border-t">
                  <JobLog jobId={j.id} status={j.status} initialOutput={j.output} />
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

function JobStatusBadge({ status }: { status: string }) {
  const m: Record<string, string> = {
    pending: "bg-yellow-100 text-yellow-700",
    running: "bg-blue-100 text-blue-700",
    done: "bg-emerald-100 text-emerald-700",
    failed: "bg-red-100 text-red-700",
  }
  return <span className={cn("text-xs px-1.5 py-0.5 rounded", m[status] ?? "bg-muted text-muted-foreground")}>{status}</span>
}

function JobLog({ jobId, status, initialOutput }: { jobId: string; status: string; initialOutput: string }) {
  const [lines, setLines] = useState<string[]>(() => initialOutput.split("\n").filter(Boolean))
  const bottomRef = useRef<HTMLDivElement>(null)
  const isTerminal = status === "done" || status === "failed"

  useEffect(() => {
    if (isTerminal) return
    const es = new EventSource(`/api/jobs/${jobId}/stream`)
    es.onmessage = (e) => {
      const newLines = e.data.split("\ndata: ").flatMap((l: string) => l.split("\n")).filter(Boolean)
      setLines((prev) => [...prev, ...newLines])
    }
    es.addEventListener("done", () => es.close())
    return () => es.close()
  }, [jobId, isTerminal])

  useEffect(() => {
    bottomRef.current?.scrollIntoView()
  }, [lines])

  return (
    <div className="bg-black text-green-400 font-mono text-xs p-4 max-h-64 overflow-y-auto">
      {lines.length === 0 && <span className="text-gray-600">{isTerminal ? "No output." : "Waiting for output…"}</span>}
      {lines.map((l, i) => <div key={i}>{l}</div>)}
      <div ref={bottomRef} />
    </div>
  )
}

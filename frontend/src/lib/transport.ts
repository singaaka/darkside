import { createConnectTransport } from "@connectrpc/connect-web"

// baseUrl is "" so the transport posts to the same origin as the SPA. In dev,
// Vite proxies the Connect paths to localhost:8080; in prod, the Go binary
// serves both the SPA and the Connect endpoints.
export const transport = createConnectTransport({
  baseUrl: "",
  // useBinaryFormat: false keeps requests as JSON, which is easier to inspect
  // in the network panel. Flip to true for production if you want smaller
  // payloads.
  useBinaryFormat: false,
})

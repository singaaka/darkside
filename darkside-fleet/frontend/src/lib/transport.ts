import { createConnectTransport } from "@connectrpc/connect-web"

export const transport = createConnectTransport({
  baseUrl: "",
  useBinaryFormat: false,
})

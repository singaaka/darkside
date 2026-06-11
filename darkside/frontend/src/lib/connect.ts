import { createClient } from "@connectrpc/connect"
import {
  HealthService,
  GitHubService,
  AppService,
  DeploymentService,
} from "@/gen/darkside/v1/api_pb"
import { transport } from "./transport"

// Plain Connect clients for non-React callers and server-streaming RPCs.
// For unary RPCs inside components, prefer @connectrpc/connect-query hooks.
export const healthClient = createClient(HealthService, transport)
export const githubClient = createClient(GitHubService, transport)
export const appsClient = createClient(AppService, transport)
export const deploymentsClient = createClient(DeploymentService, transport)

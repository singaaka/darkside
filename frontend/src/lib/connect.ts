import { createClient } from "@connectrpc/connect"
import {
  HealthService,
  GitHubService,
  AppService,
  EnvironmentService,
  DeploymentService,
} from "@/gen/darkside/v1/api_pb"
import { transport } from "./transport"

// Plain Connect clients for non-React callers (loaders, mutations outside hooks,
// server-streaming RPCs that don't fit the query model). Inside components for
// unary RPCs, prefer @connectrpc/connect-query hooks against the same service
// descriptors — same descriptor, same shape, cached.
export const healthClient = createClient(HealthService, transport)
export const githubClient = createClient(GitHubService, transport)
export const appsClient = createClient(AppService, transport)
export const envClient = createClient(EnvironmentService, transport)
export const deploymentsClient = createClient(DeploymentService, transport)

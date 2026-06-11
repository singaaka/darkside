import { createClient } from "@connectrpc/connect"
import { NodeService, ClusterConfigService, JobService, SettingsService } from "@/gen/fleet/v1/api_pb"
import { transport } from "./transport"

export const nodeClient = createClient(NodeService, transport)
export const configClient = createClient(ClusterConfigService, transport)
export const jobClient = createClient(JobService, transport)
export const settingsClient = createClient(SettingsService, transport)

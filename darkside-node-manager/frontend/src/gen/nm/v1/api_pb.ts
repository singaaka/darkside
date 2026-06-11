// @generated — protobuf-es v2 compatible bindings for nm/v1/api.proto

import { proto3, Message } from "@bufbuild/protobuf";

// ── Node ──────────────────────────────────────────────────────────────────────

export class Node extends Message<Node> {
  id = ""; name = ""; public_ip = ""; ssh_user = ""; ssh_key_path = "";
  tags: string[] = []; is_darkside_paas_node = false; status = "";
  nomad_node_id = ""; created_at_unix = BigInt(0);
  static readonly typeName = "nm.v1.Node";
  static readonly fields = proto3.util.newFieldList(() => [
    { no: 1, name: "id", kind: "scalar", T: 9 },
    { no: 2, name: "name", kind: "scalar", T: 9 },
    { no: 3, name: "public_ip", kind: "scalar", T: 9 },
    { no: 4, name: "ssh_user", kind: "scalar", T: 9 },
    { no: 5, name: "ssh_key_path", kind: "scalar", T: 9 },
    { no: 6, name: "tags", kind: "scalar", T: 9, repeated: true },
    { no: 7, name: "is_darkside_paas_node", kind: "scalar", T: 8 },
    { no: 8, name: "status", kind: "scalar", T: 9 },
    { no: 9, name: "nomad_node_id", kind: "scalar", T: 9 },
    { no: 10, name: "created_at_unix", kind: "scalar", T: 3 },
  ]);
  static fromBinary(b: Uint8Array): Node { return new Node().fromBinary(b); }
  static fromJson(j: unknown): Node { return new Node().fromJson(j); }
}

export class ListNodesRequest extends Message<ListNodesRequest> {
  static readonly typeName = "nm.v1.ListNodesRequest";
  static readonly fields = proto3.util.newFieldList(() => []);
  static fromBinary(b: Uint8Array): ListNodesRequest { return new ListNodesRequest().fromBinary(b); }
  static fromJson(j: unknown): ListNodesRequest { return new ListNodesRequest().fromJson(j); }
}

export class ListNodesResponse extends Message<ListNodesResponse> {
  nodes: Node[] = [];
  static readonly typeName = "nm.v1.ListNodesResponse";
  static readonly fields = proto3.util.newFieldList(() => [
    { no: 1, name: "nodes", kind: "message", T: Node, repeated: true },
  ]);
  static fromBinary(b: Uint8Array): ListNodesResponse { return new ListNodesResponse().fromBinary(b); }
  static fromJson(j: unknown): ListNodesResponse { return new ListNodesResponse().fromJson(j); }
}

export class GetNodeRequest extends Message<GetNodeRequest> {
  id = "";
  static readonly typeName = "nm.v1.GetNodeRequest";
  static readonly fields = proto3.util.newFieldList(() => [{ no: 1, name: "id", kind: "scalar", T: 9 }]);
  static fromBinary(b: Uint8Array): GetNodeRequest { return new GetNodeRequest().fromBinary(b); }
  static fromJson(j: unknown): GetNodeRequest { return new GetNodeRequest().fromJson(j); }
}

export class AddNodeRequest extends Message<AddNodeRequest> {
  name = ""; public_ip = ""; ssh_user = ""; ssh_key_path = ""; tags: string[] = [];
  static readonly typeName = "nm.v1.AddNodeRequest";
  static readonly fields = proto3.util.newFieldList(() => [
    { no: 1, name: "name", kind: "scalar", T: 9 },
    { no: 2, name: "public_ip", kind: "scalar", T: 9 },
    { no: 3, name: "ssh_user", kind: "scalar", T: 9 },
    { no: 4, name: "ssh_key_path", kind: "scalar", T: 9 },
    { no: 5, name: "tags", kind: "scalar", T: 9, repeated: true },
  ]);
  static fromBinary(b: Uint8Array): AddNodeRequest { return new AddNodeRequest().fromBinary(b); }
  static fromJson(j: unknown): AddNodeRequest { return new AddNodeRequest().fromJson(j); }
}

export class DeleteNodeRequest extends Message<DeleteNodeRequest> {
  id = "";
  static readonly typeName = "nm.v1.DeleteNodeRequest";
  static readonly fields = proto3.util.newFieldList(() => [{ no: 1, name: "id", kind: "scalar", T: 9 }]);
  static fromBinary(b: Uint8Array): DeleteNodeRequest { return new DeleteNodeRequest().fromBinary(b); }
  static fromJson(j: unknown): DeleteNodeRequest { return new DeleteNodeRequest().fromJson(j); }
}

export class DeleteNodeResponse extends Message<DeleteNodeResponse> {
  ok = false;
  static readonly typeName = "nm.v1.DeleteNodeResponse";
  static readonly fields = proto3.util.newFieldList(() => [{ no: 1, name: "ok", kind: "scalar", T: 8 }]);
  static fromBinary(b: Uint8Array): DeleteNodeResponse { return new DeleteNodeResponse().fromBinary(b); }
  static fromJson(j: unknown): DeleteNodeResponse { return new DeleteNodeResponse().fromJson(j); }
}

export class SetPaasNodeRequest extends Message<SetPaasNodeRequest> {
  node_id = "";
  static readonly typeName = "nm.v1.SetPaasNodeRequest";
  static readonly fields = proto3.util.newFieldList(() => [{ no: 1, name: "node_id", kind: "scalar", T: 9 }]);
  static fromBinary(b: Uint8Array): SetPaasNodeRequest { return new SetPaasNodeRequest().fromBinary(b); }
  static fromJson(j: unknown): SetPaasNodeRequest { return new SetPaasNodeRequest().fromJson(j); }
}

export const NodeService = {
  typeName: "nm.v1.NodeService",
  methods: {
    list: { name: "List", I: ListNodesRequest, O: ListNodesResponse, kind: "unary" },
    get: { name: "Get", I: GetNodeRequest, O: Node, kind: "unary" },
    add: { name: "Add", I: AddNodeRequest, O: Node, kind: "unary" },
    delete: { name: "Delete", I: DeleteNodeRequest, O: DeleteNodeResponse, kind: "unary" },
    setPaasNode: { name: "SetPaasNode", I: SetPaasNodeRequest, O: Node, kind: "unary" },
  },
} as const;

// ── ClusterConfigService ───────────────────────────────────────────────────────

export class NodeConfig extends Message<NodeConfig> {
  node_id = ""; nomad_hcl = ""; consul_hcl = ""; traefik_yml = ""; updated_at_unix = BigInt(0);
  static readonly typeName = "nm.v1.NodeConfig";
  static readonly fields = proto3.util.newFieldList(() => [
    { no: 1, name: "node_id", kind: "scalar", T: 9 },
    { no: 2, name: "nomad_hcl", kind: "scalar", T: 9 },
    { no: 3, name: "consul_hcl", kind: "scalar", T: 9 },
    { no: 4, name: "traefik_yml", kind: "scalar", T: 9 },
    { no: 5, name: "updated_at_unix", kind: "scalar", T: 3 },
  ]);
  static fromBinary(b: Uint8Array): NodeConfig { return new NodeConfig().fromBinary(b); }
  static fromJson(j: unknown): NodeConfig { return new NodeConfig().fromJson(j); }
}

export class GetNodeConfigRequest extends Message<GetNodeConfigRequest> {
  node_id = "";
  static readonly typeName = "nm.v1.GetNodeConfigRequest";
  static readonly fields = proto3.util.newFieldList(() => [{ no: 1, name: "node_id", kind: "scalar", T: 9 }]);
  static fromBinary(b: Uint8Array): GetNodeConfigRequest { return new GetNodeConfigRequest().fromBinary(b); }
  static fromJson(j: unknown): GetNodeConfigRequest { return new GetNodeConfigRequest().fromJson(j); }
}

export const ClusterConfigService = {
  typeName: "nm.v1.ClusterConfigService",
  methods: {
    getNodeConfig: { name: "GetNodeConfig", I: GetNodeConfigRequest, O: NodeConfig, kind: "unary" },
  },
} as const;

// ── JobService ────────────────────────────────────────────────────────────────

export class Job extends Message<Job> {
  id = ""; type = ""; status = ""; payload = ""; output = "";
  created_at_unix = BigInt(0); started_at_unix = BigInt(0); finished_at_unix = BigInt(0);
  static readonly typeName = "nm.v1.Job";
  static readonly fields = proto3.util.newFieldList(() => [
    { no: 1, name: "id", kind: "scalar", T: 9 },
    { no: 2, name: "type", kind: "scalar", T: 9 },
    { no: 3, name: "status", kind: "scalar", T: 9 },
    { no: 4, name: "payload", kind: "scalar", T: 9 },
    { no: 5, name: "output", kind: "scalar", T: 9 },
    { no: 6, name: "created_at_unix", kind: "scalar", T: 3 },
    { no: 7, name: "started_at_unix", kind: "scalar", T: 3 },
    { no: 8, name: "finished_at_unix", kind: "scalar", T: 3 },
  ]);
  static fromBinary(b: Uint8Array): Job { return new Job().fromBinary(b); }
  static fromJson(j: unknown): Job { return new Job().fromJson(j); }
}

export class ListJobsRequest extends Message<ListJobsRequest> {
  limit = 0;
  static readonly typeName = "nm.v1.ListJobsRequest";
  static readonly fields = proto3.util.newFieldList(() => [{ no: 1, name: "limit", kind: "scalar", T: 5 }]);
  static fromBinary(b: Uint8Array): ListJobsRequest { return new ListJobsRequest().fromBinary(b); }
  static fromJson(j: unknown): ListJobsRequest { return new ListJobsRequest().fromJson(j); }
}

export class ListJobsResponse extends Message<ListJobsResponse> {
  jobs: Job[] = [];
  static readonly typeName = "nm.v1.ListJobsResponse";
  static readonly fields = proto3.util.newFieldList(() => [
    { no: 1, name: "jobs", kind: "message", T: Job, repeated: true },
  ]);
  static fromBinary(b: Uint8Array): ListJobsResponse { return new ListJobsResponse().fromBinary(b); }
  static fromJson(j: unknown): ListJobsResponse { return new ListJobsResponse().fromJson(j); }
}

export class GetJobRequest extends Message<GetJobRequest> {
  id = "";
  static readonly typeName = "nm.v1.GetJobRequest";
  static readonly fields = proto3.util.newFieldList(() => [{ no: 1, name: "id", kind: "scalar", T: 9 }]);
  static fromBinary(b: Uint8Array): GetJobRequest { return new GetJobRequest().fromBinary(b); }
  static fromJson(j: unknown): GetJobRequest { return new GetJobRequest().fromJson(j); }
}

export const JobService = {
  typeName: "nm.v1.JobService",
  methods: {
    list: { name: "List", I: ListJobsRequest, O: ListJobsResponse, kind: "unary" },
    get: { name: "Get", I: GetJobRequest, O: Job, kind: "unary" },
  },
} as const;

// ── SettingsService ───────────────────────────────────────────────────────────

export class Settings extends Message<Settings> {
  domain = ""; registry_port = 0; darkside_paas_node_id = "";
  static readonly typeName = "nm.v1.Settings";
  static readonly fields = proto3.util.newFieldList(() => [
    { no: 1, name: "domain", kind: "scalar", T: 9 },
    { no: 2, name: "registry_port", kind: "scalar", T: 5 },
    { no: 3, name: "darkside_paas_node_id", kind: "scalar", T: 9 },
  ]);
  static fromBinary(b: Uint8Array): Settings { return new Settings().fromBinary(b); }
  static fromJson(j: unknown): Settings { return new Settings().fromJson(j); }
}

export class GetSettingsRequest extends Message<GetSettingsRequest> {
  static readonly typeName = "nm.v1.GetSettingsRequest";
  static readonly fields = proto3.util.newFieldList(() => []);
  static fromBinary(b: Uint8Array): GetSettingsRequest { return new GetSettingsRequest().fromBinary(b); }
  static fromJson(j: unknown): GetSettingsRequest { return new GetSettingsRequest().fromJson(j); }
}

export class UpdateSettingsRequest extends Message<UpdateSettingsRequest> {
  domain = ""; registry_port = 0;
  static readonly typeName = "nm.v1.UpdateSettingsRequest";
  static readonly fields = proto3.util.newFieldList(() => [
    { no: 1, name: "domain", kind: "scalar", T: 9 },
    { no: 2, name: "registry_port", kind: "scalar", T: 5 },
  ]);
  static fromBinary(b: Uint8Array): UpdateSettingsRequest { return new UpdateSettingsRequest().fromBinary(b); }
  static fromJson(j: unknown): UpdateSettingsRequest { return new UpdateSettingsRequest().fromJson(j); }
}

export const SettingsService = {
  typeName: "nm.v1.SettingsService",
  methods: {
    get: { name: "Get", I: GetSettingsRequest, O: Settings, kind: "unary" },
    update: { name: "Update", I: UpdateSettingsRequest, O: Settings, kind: "unary" },
  },
} as const;

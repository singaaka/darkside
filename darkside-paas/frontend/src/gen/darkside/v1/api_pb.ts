// @generated — protobuf-es v2 compatible bindings for darkside/v1/api.proto
// This file is manually maintained to match api.proto without running buf.

import { proto3, Message, type MessageShape } from "@bufbuild/protobuf";

// ── HealthService ────────────────────────────────────────────────────────────

export class PingRequest extends Message<PingRequest> {
  static readonly typeName = "darkside.v1.PingRequest";
  static readonly fields = proto3.util.newFieldList(() => []);
  static fromBinary(bytes: Uint8Array): PingRequest { return new PingRequest().fromBinary(bytes); }
  static fromJson(json: unknown): PingRequest { return new PingRequest().fromJson(json); }
}

export class PingResponse extends Message<PingResponse> {
  message = "";
  timestamp_unix = BigInt(0);
  static readonly typeName = "darkside.v1.PingResponse";
  static readonly fields = proto3.util.newFieldList(() => [
    { no: 1, name: "message", kind: "scalar", T: 9 },
    { no: 2, name: "timestamp_unix", kind: "scalar", T: 3 },
  ]);
  static fromBinary(bytes: Uint8Array): PingResponse { return new PingResponse().fromBinary(bytes); }
  static fromJson(json: unknown): PingResponse { return new PingResponse().fromJson(json); }
}

export class InfoRequest extends Message<InfoRequest> {
  static readonly typeName = "darkside.v1.InfoRequest";
  static readonly fields = proto3.util.newFieldList(() => []);
  static fromBinary(bytes: Uint8Array): InfoRequest { return new InfoRequest().fromBinary(bytes); }
  static fromJson(json: unknown): InfoRequest { return new InfoRequest().fromJson(json); }
}

export class InfoResponse extends Message<InfoResponse> {
  version = "";
  domain = "";
  static readonly typeName = "darkside.v1.InfoResponse";
  static readonly fields = proto3.util.newFieldList(() => [
    { no: 1, name: "version", kind: "scalar", T: 9 },
    { no: 2, name: "domain", kind: "scalar", T: 9 },
  ]);
  static fromBinary(bytes: Uint8Array): InfoResponse { return new InfoResponse().fromBinary(bytes); }
  static fromJson(json: unknown): InfoResponse { return new InfoResponse().fromJson(json); }
}

export const HealthService = {
  typeName: "darkside.v1.HealthService",
  methods: {
    ping: { name: "Ping", I: PingRequest, O: PingResponse, kind: "unary" },
    info: { name: "Info", I: InfoRequest, O: InfoResponse, kind: "unary" },
  },
} as const;

// ── GitHubService ─────────────────────────────────────────────────────────────

export class GetGitHubStatusRequest extends Message<GetGitHubStatusRequest> {
  static readonly typeName = "darkside.v1.GetGitHubStatusRequest";
  static readonly fields = proto3.util.newFieldList(() => []);
  static fromBinary(bytes: Uint8Array): GetGitHubStatusRequest { return new GetGitHubStatusRequest().fromBinary(bytes); }
  static fromJson(json: unknown): GetGitHubStatusRequest { return new GetGitHubStatusRequest().fromJson(json); }
}

export class GetGitHubStatusResponse extends Message<GetGitHubStatusResponse> {
  connected = false;
  app_id = BigInt(0);
  app_slug = "";
  app_name = "";
  html_url = "";
  install_url = "";
  static readonly typeName = "darkside.v1.GetGitHubStatusResponse";
  static readonly fields = proto3.util.newFieldList(() => [
    { no: 1, name: "connected", kind: "scalar", T: 8 },
    { no: 2, name: "app_id", kind: "scalar", T: 3 },
    { no: 3, name: "app_slug", kind: "scalar", T: 9 },
    { no: 4, name: "app_name", kind: "scalar", T: 9 },
    { no: 5, name: "html_url", kind: "scalar", T: 9 },
    { no: 6, name: "install_url", kind: "scalar", T: 9 },
  ]);
  static fromBinary(bytes: Uint8Array): GetGitHubStatusResponse { return new GetGitHubStatusResponse().fromBinary(bytes); }
  static fromJson(json: unknown): GetGitHubStatusResponse { return new GetGitHubStatusResponse().fromJson(json); }
}

export class Installation extends Message<Installation> {
  id = BigInt(0);
  account_login = "";
  account_type = "";
  static readonly typeName = "darkside.v1.Installation";
  static readonly fields = proto3.util.newFieldList(() => [
    { no: 1, name: "id", kind: "scalar", T: 3 },
    { no: 2, name: "account_login", kind: "scalar", T: 9 },
    { no: 3, name: "account_type", kind: "scalar", T: 9 },
  ]);
  static fromBinary(bytes: Uint8Array): Installation { return new Installation().fromBinary(bytes); }
  static fromJson(json: unknown): Installation { return new Installation().fromJson(json); }
}

export class ListInstallationsRequest extends Message<ListInstallationsRequest> {
  static readonly typeName = "darkside.v1.ListInstallationsRequest";
  static readonly fields = proto3.util.newFieldList(() => []);
  static fromBinary(bytes: Uint8Array): ListInstallationsRequest { return new ListInstallationsRequest().fromBinary(bytes); }
  static fromJson(json: unknown): ListInstallationsRequest { return new ListInstallationsRequest().fromJson(json); }
}

export class ListInstallationsResponse extends Message<ListInstallationsResponse> {
  installations: Installation[] = [];
  static readonly typeName = "darkside.v1.ListInstallationsResponse";
  static readonly fields = proto3.util.newFieldList(() => [
    { no: 1, name: "installations", kind: "message", T: Installation, repeated: true },
  ]);
  static fromBinary(bytes: Uint8Array): ListInstallationsResponse { return new ListInstallationsResponse().fromBinary(bytes); }
  static fromJson(json: unknown): ListInstallationsResponse { return new ListInstallationsResponse().fromJson(json); }
}

export class Repo extends Message<Repo> {
  id = BigInt(0);
  full_name = "";
  default_branch = "";
  private = false;
  description = "";
  static readonly typeName = "darkside.v1.Repo";
  static readonly fields = proto3.util.newFieldList(() => [
    { no: 1, name: "id", kind: "scalar", T: 3 },
    { no: 2, name: "full_name", kind: "scalar", T: 9 },
    { no: 3, name: "default_branch", kind: "scalar", T: 9 },
    { no: 4, name: "private", kind: "scalar", T: 8 },
    { no: 5, name: "description", kind: "scalar", T: 9 },
  ]);
  static fromBinary(bytes: Uint8Array): Repo { return new Repo().fromBinary(bytes); }
  static fromJson(json: unknown): Repo { return new Repo().fromJson(json); }
}

export class ListInstallationReposRequest extends Message<ListInstallationReposRequest> {
  installation_id = BigInt(0);
  static readonly typeName = "darkside.v1.ListInstallationReposRequest";
  static readonly fields = proto3.util.newFieldList(() => [
    { no: 1, name: "installation_id", kind: "scalar", T: 3 },
  ]);
  static fromBinary(bytes: Uint8Array): ListInstallationReposRequest { return new ListInstallationReposRequest().fromBinary(bytes); }
  static fromJson(json: unknown): ListInstallationReposRequest { return new ListInstallationReposRequest().fromJson(json); }
}

export class ListInstallationReposResponse extends Message<ListInstallationReposResponse> {
  repos: Repo[] = [];
  static readonly typeName = "darkside.v1.ListInstallationReposResponse";
  static readonly fields = proto3.util.newFieldList(() => [
    { no: 1, name: "repos", kind: "message", T: Repo, repeated: true },
  ]);
  static fromBinary(bytes: Uint8Array): ListInstallationReposResponse { return new ListInstallationReposResponse().fromBinary(bytes); }
  static fromJson(json: unknown): ListInstallationReposResponse { return new ListInstallationReposResponse().fromJson(json); }
}

export const GitHubService = {
  typeName: "darkside.v1.GitHubService",
  methods: {
    getStatus: { name: "GetStatus", I: GetGitHubStatusRequest, O: GetGitHubStatusResponse, kind: "unary" },
    listInstallations: { name: "ListInstallations", I: ListInstallationsRequest, O: ListInstallationsResponse, kind: "unary" },
    listInstallationRepos: { name: "ListInstallationRepos", I: ListInstallationReposRequest, O: ListInstallationReposResponse, kind: "unary" },
  },
} as const;

// ── AppService ────────────────────────────────────────────────────────────────

export class App extends Message<App> {
  id = "";
  name = "";
  repo_full_name = "";
  installation_id = BigInt(0);
  created_at_unix = BigInt(0);
  static readonly typeName = "darkside.v1.App";
  static readonly fields = proto3.util.newFieldList(() => [
    { no: 1, name: "id", kind: "scalar", T: 9 },
    { no: 2, name: "name", kind: "scalar", T: 9 },
    { no: 3, name: "repo_full_name", kind: "scalar", T: 9 },
    { no: 4, name: "installation_id", kind: "scalar", T: 3 },
    { no: 5, name: "created_at_unix", kind: "scalar", T: 3 },
  ]);
  static fromBinary(bytes: Uint8Array): App { return new App().fromBinary(bytes); }
  static fromJson(json: unknown): App { return new App().fromJson(json); }
}

export class ListAppsRequest extends Message<ListAppsRequest> {
  static readonly typeName = "darkside.v1.ListAppsRequest";
  static readonly fields = proto3.util.newFieldList(() => []);
  static fromBinary(bytes: Uint8Array): ListAppsRequest { return new ListAppsRequest().fromBinary(bytes); }
  static fromJson(json: unknown): ListAppsRequest { return new ListAppsRequest().fromJson(json); }
}

export class ListAppsResponse extends Message<ListAppsResponse> {
  apps: App[] = [];
  static readonly typeName = "darkside.v1.ListAppsResponse";
  static readonly fields = proto3.util.newFieldList(() => [
    { no: 1, name: "apps", kind: "message", T: App, repeated: true },
  ]);
  static fromBinary(bytes: Uint8Array): ListAppsResponse { return new ListAppsResponse().fromBinary(bytes); }
  static fromJson(json: unknown): ListAppsResponse { return new ListAppsResponse().fromJson(json); }
}

export class GetAppRequest extends Message<GetAppRequest> {
  id = "";
  static readonly typeName = "darkside.v1.GetAppRequest";
  static readonly fields = proto3.util.newFieldList(() => [
    { no: 1, name: "id", kind: "scalar", T: 9 },
  ]);
  static fromBinary(bytes: Uint8Array): GetAppRequest { return new GetAppRequest().fromBinary(bytes); }
  static fromJson(json: unknown): GetAppRequest { return new GetAppRequest().fromJson(json); }
}

export class CreateAppRequest extends Message<CreateAppRequest> {
  name = "";
  repo_full_name = "";
  installation_id = BigInt(0);
  static readonly typeName = "darkside.v1.CreateAppRequest";
  static readonly fields = proto3.util.newFieldList(() => [
    { no: 1, name: "name", kind: "scalar", T: 9 },
    { no: 2, name: "repo_full_name", kind: "scalar", T: 9 },
    { no: 3, name: "installation_id", kind: "scalar", T: 3 },
  ]);
  static fromBinary(bytes: Uint8Array): CreateAppRequest { return new CreateAppRequest().fromBinary(bytes); }
  static fromJson(json: unknown): CreateAppRequest { return new CreateAppRequest().fromJson(json); }
}

export class GetManifestSampleRequest extends Message<GetManifestSampleRequest> {
  app_id = "";
  static readonly typeName = "darkside.v1.GetManifestSampleRequest";
  static readonly fields = proto3.util.newFieldList(() => [
    { no: 1, name: "app_id", kind: "scalar", T: 9 },
  ]);
  static fromBinary(bytes: Uint8Array): GetManifestSampleRequest { return new GetManifestSampleRequest().fromBinary(bytes); }
  static fromJson(json: unknown): GetManifestSampleRequest { return new GetManifestSampleRequest().fromJson(json); }
}

export class GetManifestSampleResponse extends Message<GetManifestSampleResponse> {
  toml = "";
  static readonly typeName = "darkside.v1.GetManifestSampleResponse";
  static readonly fields = proto3.util.newFieldList(() => [
    { no: 1, name: "toml", kind: "scalar", T: 9 },
  ]);
  static fromBinary(bytes: Uint8Array): GetManifestSampleResponse { return new GetManifestSampleResponse().fromBinary(bytes); }
  static fromJson(json: unknown): GetManifestSampleResponse { return new GetManifestSampleResponse().fromJson(json); }
}

export const AppService = {
  typeName: "darkside.v1.AppService",
  methods: {
    list: { name: "List", I: ListAppsRequest, O: ListAppsResponse, kind: "unary" },
    get: { name: "Get", I: GetAppRequest, O: App, kind: "unary" },
    create: { name: "Create", I: CreateAppRequest, O: App, kind: "unary" },
    getManifestSample: { name: "GetManifestSample", I: GetManifestSampleRequest, O: GetManifestSampleResponse, kind: "unary" },
  },
} as const;

// ── DeploymentService ─────────────────────────────────────────────────────────

export class Deployment extends Message<Deployment> {
  id = "";
  app_id = "";
  commit_sha = "";
  commit_message = "";
  image_tag = "";
  status = "";
  nomad_job_hcl = "";
  env_snapshot = "";
  error = "";
  started_at_unix = BigInt(0);
  finished_at_unix = BigInt(0);
  static readonly typeName = "darkside.v1.Deployment";
  static readonly fields = proto3.util.newFieldList(() => [
    { no: 1, name: "id", kind: "scalar", T: 9 },
    { no: 2, name: "app_id", kind: "scalar", T: 9 },
    { no: 5, name: "commit_sha", kind: "scalar", T: 9 },
    { no: 6, name: "commit_message", kind: "scalar", T: 9 },
    { no: 7, name: "image_tag", kind: "scalar", T: 9 },
    { no: 8, name: "status", kind: "scalar", T: 9 },
    { no: 9, name: "nomad_job_hcl", kind: "scalar", T: 9 },
    { no: 10, name: "env_snapshot", kind: "scalar", T: 9 },
    { no: 11, name: "error", kind: "scalar", T: 9 },
    { no: 12, name: "started_at_unix", kind: "scalar", T: 3 },
    { no: 13, name: "finished_at_unix", kind: "scalar", T: 3 },
  ]);
  static fromBinary(bytes: Uint8Array): Deployment { return new Deployment().fromBinary(bytes); }
  static fromJson(json: unknown): Deployment { return new Deployment().fromJson(json); }
}

export class ListDeploymentsRequest extends Message<ListDeploymentsRequest> {
  app_id = "";
  limit = 0;
  static readonly typeName = "darkside.v1.ListDeploymentsRequest";
  static readonly fields = proto3.util.newFieldList(() => [
    { no: 1, name: "app_id", kind: "scalar", T: 9 },
    { no: 2, name: "limit", kind: "scalar", T: 5 },
  ]);
  static fromBinary(bytes: Uint8Array): ListDeploymentsRequest { return new ListDeploymentsRequest().fromBinary(bytes); }
  static fromJson(json: unknown): ListDeploymentsRequest { return new ListDeploymentsRequest().fromJson(json); }
}

export class ListDeploymentsResponse extends Message<ListDeploymentsResponse> {
  deployments: Deployment[] = [];
  static readonly typeName = "darkside.v1.ListDeploymentsResponse";
  static readonly fields = proto3.util.newFieldList(() => [
    { no: 1, name: "deployments", kind: "message", T: Deployment, repeated: true },
  ]);
  static fromBinary(bytes: Uint8Array): ListDeploymentsResponse { return new ListDeploymentsResponse().fromBinary(bytes); }
  static fromJson(json: unknown): ListDeploymentsResponse { return new ListDeploymentsResponse().fromJson(json); }
}

export class GetDeploymentRequest extends Message<GetDeploymentRequest> {
  id = "";
  static readonly typeName = "darkside.v1.GetDeploymentRequest";
  static readonly fields = proto3.util.newFieldList(() => [
    { no: 1, name: "id", kind: "scalar", T: 9 },
  ]);
  static fromBinary(bytes: Uint8Array): GetDeploymentRequest { return new GetDeploymentRequest().fromBinary(bytes); }
  static fromJson(json: unknown): GetDeploymentRequest { return new GetDeploymentRequest().fromJson(json); }
}

export class StreamLogsRequest extends Message<StreamLogsRequest> {
  deployment_id = "";
  phase = "";
  static readonly typeName = "darkside.v1.StreamLogsRequest";
  static readonly fields = proto3.util.newFieldList(() => [
    { no: 1, name: "deployment_id", kind: "scalar", T: 9 },
    { no: 2, name: "phase", kind: "scalar", T: 9 },
  ]);
  static fromBinary(bytes: Uint8Array): StreamLogsRequest { return new StreamLogsRequest().fromBinary(bytes); }
  static fromJson(json: unknown): StreamLogsRequest { return new StreamLogsRequest().fromJson(json); }
}

export class StreamLogsResponse extends Message<StreamLogsResponse> {
  chunk = "";
  static readonly typeName = "darkside.v1.StreamLogsResponse";
  static readonly fields = proto3.util.newFieldList(() => [
    { no: 1, name: "chunk", kind: "scalar", T: 9 },
  ]);
  static fromBinary(bytes: Uint8Array): StreamLogsResponse { return new StreamLogsResponse().fromBinary(bytes); }
  static fromJson(json: unknown): StreamLogsResponse { return new StreamLogsResponse().fromJson(json); }
}

export class RedeployRequest extends Message<RedeployRequest> {
  deployment_id = "";
  static readonly typeName = "darkside.v1.RedeployRequest";
  static readonly fields = proto3.util.newFieldList(() => [
    { no: 1, name: "deployment_id", kind: "scalar", T: 9 },
  ]);
  static fromBinary(bytes: Uint8Array): RedeployRequest { return new RedeployRequest().fromBinary(bytes); }
  static fromJson(json: unknown): RedeployRequest { return new RedeployRequest().fromJson(json); }
}

export const DeploymentService = {
  typeName: "darkside.v1.DeploymentService",
  methods: {
    list: { name: "List", I: ListDeploymentsRequest, O: ListDeploymentsResponse, kind: "unary" },
    get: { name: "Get", I: GetDeploymentRequest, O: Deployment, kind: "unary" },
    streamLogs: { name: "StreamLogs", I: StreamLogsRequest, O: StreamLogsResponse, kind: "server_stream" },
    redeploy: { name: "Redeploy", I: RedeployRequest, O: Deployment, kind: "unary" },
  },
} as const;

// Re-export the file descriptor placeholder used by connect handlers.
// The actual binary descriptor isn't needed when useBinaryFormat=false.
export const File_darkside_v1_api_proto = {
  typeName: "darkside/v1/api.proto",
  Services: () => ({
    ByName: (name: string) => ({
      Methods: () => ({ ByName: (_: string) => undefined }),
    }),
  }),
} as any;

// Type helpers
export type AppDetail = {
  id: string
  name: string
  repo_full_name: string
  installation_id: number
  created_at: number
  branch: string
  env_file: string
  age_public_key: string
  age_key_id: string
  age_private_key?: string
  setup_instructions?: string
}

export type DeploymentDetail = {
  id: string
  app_id: string
  commit_sha: string
  commit_message: string
  status: string
  trigger_type: "github" | "manual" | "rollback"
  trigger_branch?: string
  image_tag?: string
  nomad_job_hcl?: string
  build_job_hcl?: string
  pre_hook_hcl?: string
  post_hook_hcl?: string
  env_snapshot?: string
  error?: string
  started_at: number
  finished_at?: number
}

export type UserProfile = {
  id: number
  login: string
  email: string
  is_admin: boolean
  whitelisted: boolean
}

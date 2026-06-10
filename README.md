# darkside

A self-hosted PaaS: `nomad` + `traefik` + `darkside`, one `docker compose up`.

The pitch: everything specific to darkside lives in your repo as `darkside.toml`.
On push, darkside clones, builds an image, runs your pre hook, performs a
rolling deploy via nomad, runs your post hook. Traefik routes traffic based on
service tags you supply in the manifest. Environments use `age`-encrypted env
files; the public keys ship with your repo, the private keys live in darkside.

## Setup

```bash
cp darkside.example.toml darkside.toml   # edit `domain` and `external_url`
cp .env.example .env                     # edit DOMAIN + DARKSIDE_BASIC_AUTH
docker compose up -d --build
```

Then point `darkside.<DOMAIN>` and `*.<DOMAIN>` DNS at the host. For local dev,
add `darkside.example.com` and your app subdomains to `/etc/hosts`.

The dashboard sits at `https://darkside.<DOMAIN>/` behind HTTP basic auth.
Generate the credentials with:

```bash
htpasswd -nbB admin yourpassword
# In .env, escape every $ as $$ — docker-compose expands single $.
```

## Day-one flow

1. Open the dashboard → **Settings** → **Connect GitHub**. The manifest flow
   redirects to github.com to provision a GitHub App; on return darkside stores
   the credentials.
2. Install the App on whichever org/repos you want to deploy.
3. **Apps → New app** → pick installation + repo → set a slug.
4. On the new app's page:
   - **Manifest** tab → copy the pre-filled `darkside.toml` into your repo.
   - **Environments** tab → create `production` (or whatever). darkside
     generates an age keypair and hands you bash commands to save the private
     key locally, commit the recipient, and encrypt `env.production.age`.
5. Commit everything; push to the branch you mapped in `[branches]`.
6. The dashboard's **Deployments** tab shows the build in flight with live
   logs. After build, you'll see streams for the pre hook, the nomad rolling
   deploy, and the post hook.

## Architecture

```
┌───────────────────────────────────────────────────────────────┐
│                       docker compose                          │
│                                                               │
│  ┌─────────┐    ┌──────────────────────┐    ┌──────────────┐  │
│  │ traefik │───▶│   darkside (Go +     │───▶│    nomad     │  │
│  │  :80    │    │   embedded React)    │    │    :4646     │  │
│  │  :443   │    │   :8080              │    │ (host net)   │  │
│  └─────────┘    └──────────────────────┘    └──────────────┘  │
│       │                   │                       │           │
│       │                   ▼                       ▼           │
│       │             ┌─────────────┐         /var/run/         │
│       │             │ sqlite      │         docker.sock       │
│       │             │ (in volume) │            │              │
│       │             └─────────────┘            ▼              │
│       │                                  ┌──────────┐         │
│       └─────────────────────────────────▶│  apps    │         │
│         (routes via nomad service tags)  └──────────┘         │
└───────────────────────────────────────────────────────────────┘
```

- Traefik reads service tags from nomad (`provider = "nomad"` in each service
  block) to discover where to route per-app traffic.
- The webhook endpoint at `/webhooks/github` sits in a higher-priority traefik
  router that bypasses basic-auth.
- darkside drives docker directly for image builds + pre/post hooks. Nomad
  handles the long-lived workloads.
- All build/deploy logs stream over a ConnectRPC server-stream RPC and are
  persisted in sqlite once each phase completes.

## Repo layout

```
cmd/darkside/        Go entrypoint (h2c-wrapped HTTP server)
internal/
  config/            operator config loader (darkside.toml at /etc/darkside)
  db/                goose migrations (embedded) + sqlc-generated queries (dbgen)
  store/             thin sqlc wrapper
  server/            ConnectRPC handlers + HTTP routes
  github/            GitHub App JWT + API client (manifest exchange, repos)
  manifest/          darkside.toml parser + sample generator
  ageenv/            age keypair generation + decrypt + env file parsing
  loghub/            in-memory pub/sub for live phase logs
  buildqueue/        per-app FIFO worker queue
  builder/           clone → docker build (hands off to deployer)
  deployer/          decrypt env → pre hook → nomad render+submit → post hook
  frontend/          embed.FS of frontend/dist
proto/darkside/v1/   ConnectRPC source of truth
gen/                 generated Go proto bindings (gitignored)
configs/             nomad agent + traefik config
frontend/            Vite + React + TanStack Router + Query + shadcn
```

## Local development without docker

```bash
just proto              # generate Go + TS proto bindings (needs buf)
just sqlc               # generate Go DB layer (needs sqlc; output is committed)
just dev-backend        # in one shell — backend at :8080
just dev-frontend       # in another — vite dev server at :5173 (proxies API)
```

`just` is a task runner (`brew install just` on macOS). Run `just` with no
args to see all recipes. The full toolchain is: `just`, `buf`, `sqlc`, `goose`,
`npm`, Go 1.25+, the docker daemon, and (for actually deploying) a nomad agent
reachable at `nomad_addr` from the config.

### Migrations

Schema changes live in `internal/db/migrations/` as versioned `.sql` files
managed by [pressly/goose](https://github.com/pressly/goose). Migrations are
embedded into the binary via `embed.FS` and applied at startup — no separate
migrate step in deploys, no volume mount for migration files.

```bash
just new-migration add_app_owner   # writes internal/db/migrations/00002_add_app_owner.sql
# edit the file, then:
just sqlc                          # regenerate the typed query layer
```

The schema_migrations table tracks applied versions; rerunning the binary on
an up-to-date DB is a no-op.

## darkside.toml schema

The app-side manifest. The dashboard's Manifest tab generates a copy-paste
starter pre-filled with your app name and host.

```toml
name = "my-app"

[build]
context    = "."
dockerfile = "Dockerfile"

[deploy]
count = 1
traefik_tags = [
  "traefik.enable=true",
  "traefik.http.routers.my-app.rule=Host(`my-app.example.com`)",
  "traefik.http.routers.my-app.entrypoints=web",
  "traefik.http.services.my-app.loadbalancer.server.port=8080",
]

[health_check]
path     = "/healthz"
port     = 8080
interval = "30s"
timeout  = "5s"

[hooks]
pre  = "./scripts/migrate.sh"   # optional
post = "./scripts/notify.sh"    # optional
run  = ""                       # optional, overrides image CMD in nomad

[branches]
main = "production"             # push to main → deploy production

[[environments]]
name     = "production"
env_file = "env.production.age"
```

## Caveats / known gaps

- **Age private keys are stored in sqlite unencrypted.** Per spec — operators
  asked for full visibility at this stage. Wrap with a master key later.
- **No multi-host nomad.** Single-node dev cluster on host networking. Multi-
  host needs an image registry (we currently build local-only) and proper
  nomad clustering.
- **One app per repo.** No monorepo discovery in v1.
- **Build queue is in-memory.** Crashes lose pending jobs; the next push picks
  up.

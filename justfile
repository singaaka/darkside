# darkside task runner. Install just with `brew install just`.
# Run `just` (no args) to see this list.

# Default: list recipes.
default:
    @just --list

# Generate Go + TS bindings from .proto sources. Needs `buf` on PATH.
proto:
    buf generate

# Generate the typed sqlite query layer. Needs `sqlc` on PATH.
sqlc:
    sqlc generate

# Create a new versioned migration. Pass a snake_case name, e.g. `just new-migration add_app_owner`.
new-migration name:
    goose -dir internal/db/migrations create {{name}} sql

# Tidy Go module deps.
tidy:
    go mod tidy

# Run the backend locally. Expects darkside.toml in the repo root.
dev-backend:
    DARKSIDE_CONFIG=./darkside.toml go run ./cmd/darkside

# Run the Vite dev server. Proxies /darkside.v1.* and /webhooks to :8080.
dev-frontend:
    cd frontend && npm run dev

# Build a standalone Go binary with the frontend embedded. Requires `proto`
# to have run at least once so gen/ exists.
build-binary:
    cd frontend && npm install && npm run build
    rm -rf internal/frontend/dist
    cp -r frontend/dist internal/frontend/dist
    go build -o bin/darkside ./cmd/darkside

# Build all docker images.
build:
    docker compose build

# Start the stack in the background.
up:
    docker compose up -d

# Stop everything.
down:
    docker compose down

# Tail the darkside container's logs.
logs:
    docker compose logs -f darkside

# Wipe generated artifacts. Keeps your darkside.toml and ./data.
clean:
    rm -rf gen/ internal/frontend/dist frontend/dist frontend/src/gen frontend/node_modules bin/

# Vet + build a throwaway binary as a fast pre-commit check.
check:
    go vet ./...
    go build -o /dev/null ./cmd/darkside

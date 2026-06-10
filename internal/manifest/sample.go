package manifest

import (
	"fmt"
	"strings"
)

// Sample returns a copy-pasteable darkside.toml prefilled with the app's name.
// Traefik routing is intentionally LEFT AS PLACEHOLDERS — darkside no longer
// assumes any subdomain convention, so the user writes whatever Host rule
// (path-based, host-based, multiple hosts, none) suits their app.
func Sample(appName string) string {
	bt := "`" // Traefik's Host(`x`) syntax needs literal backticks; isolate them.

	var b strings.Builder
	w := func(s string) { b.WriteString(s); b.WriteByte('\n') }

	w("# darkside.toml — paste at the root of your repo.")
	w("")
	w(fmt.Sprintf("name = %q", appName))
	w("")
	w("[build]")
	w(`# Build context relative to the repo root. Dockerfile is resolved within it.`)
	w(`context = "."`)
	w(`dockerfile = "Dockerfile"`)
	w("")
	w("[deploy]")
	w("# Number of running instances.")
	w("count = 1")
	w("")
	w("# Traefik labels — passed through to nomad as service tags. Replace the")
	w("# host names + port to fit your app. darkside does NOT auto-route apps to")
	w("# any subdomain; you wire it up explicitly here.")
	w("traefik_tags = [")
	w(`  "traefik.enable=true",`)
	w("")
	w(`  # Plain HTTP on port 80. Replace YOUR-HOST below.`)
	w(fmt.Sprintf(`  "traefik.http.routers.%s.rule=Host(%sYOUR-HOST%s)",`, appName, bt, bt))
	w(fmt.Sprintf(`  "traefik.http.routers.%s.entrypoints=web",`, appName))
	w(fmt.Sprintf(`  "traefik.http.services.%s.loadbalancer.server.port=8080",`, appName))
	w("")
	w(`  # HTTPS via the letsencrypt resolver configured in traefik. Uncomment:`)
	w(fmt.Sprintf(`  # "traefik.http.routers.%s-tls.rule=Host(%sYOUR-HOST%s)",`, appName, bt, bt))
	w(fmt.Sprintf(`  # "traefik.http.routers.%s-tls.entrypoints=websecure",`, appName))
	w(fmt.Sprintf(`  # "traefik.http.routers.%s-tls.tls.certresolver=letsencrypt",`, appName))
	w("")
	w(`  # Force-redirect HTTP → HTTPS (uncomment with the TLS block above):`)
	w(fmt.Sprintf(`  # "traefik.http.routers.%s.middlewares=%s-https",`, appName, appName))
	w(fmt.Sprintf(`  # "traefik.http.middlewares.%s-https.redirectscheme.scheme=https",`, appName))
	w("")
	w(`  # Path-based routing instead of a dedicated host (uncomment as needed):`)
	w(fmt.Sprintf(`  # "traefik.http.routers.%s.rule=PathPrefix(%s/api/%s)",`, appName, bt, bt))
	w("]")
	w("")
	w("[health_check]")
	w("# Used by nomad to gate deployments. Write a real /healthz endpoint when you can.")
	w(`path = "/"`)
	w("port = 8080")
	w(`interval = "30s"`)
	w(`timeout = "5s"`)
	w("")
	w("[hooks]")
	w("# Optional. Each command runs in a `docker run --rm` against the built image,")
	w("# with the resolved env vars injected. The pre hook runs BEFORE the rolling")
	w("# deploy; the post hook runs AFTER it is healthy. Both fail-fast.")
	w(`# pre  = "./scripts/migrate.sh"`)
	w(`# post = "./scripts/notify.sh"`)
	w("")
	w("# run overrides the image's CMD when nomad launches the long-lived process.")
	w(`# run  = ""`)
	w("")
	w("[branches]")
	w("# Push → environment. Add lines for additional branches.")
	w(`main = "production"`)
	w("")
	w("# At least one [[environments]] block is required. Create the env in the")
	w("# darkside dashboard first — it'll generate the keypair and give you bash")
	w("# commands to encrypt env.<name>.age.")
	w("[[environments]]")
	w(`name = "production"`)
	w(`env_file = "env.production.age"`)

	return b.String()
}

package manifest

import (
	"fmt"
	"strings"
)

// Sample returns a copy-pasteable darkside.toml prefilled with the app's name
// and a default Host rule under <domain>. Comments explain HTTP/HTTPS choices
// so the user can edit-in-place.
func Sample(appName, domain string) string {
	host := appName + "." + domain
	bt := "`" // Traefik's host syntax needs literal backticks; isolate them.

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
	w("# Traefik labels — passed through to nomad as service tags. Edit the host")
	w("# name + port to match your app. Comments below show HTTPS additions.")
	w("traefik_tags = [")
	w(`  "traefik.enable=true",`)
	w("")
	w(`  # Plain HTTP on port 80.`)
	w(fmt.Sprintf(`  "traefik.http.routers.%s.rule=Host(%s%s%s)",`, appName, bt, host, bt))
	w(fmt.Sprintf(`  "traefik.http.routers.%s.entrypoints=web",`, appName))
	w(fmt.Sprintf(`  "traefik.http.services.%s.loadbalancer.server.port=8080",`, appName))
	w("")
	w(`  # For HTTPS via a letsencrypt cert resolver (configure the resolver in`)
	w(`  # traefik static config first), uncomment:`)
	w(fmt.Sprintf(`  # "traefik.http.routers.%s-tls.rule=Host(%s%s%s)",`, appName, bt, host, bt))
	w(fmt.Sprintf(`  # "traefik.http.routers.%s-tls.entrypoints=websecure",`, appName))
	w(fmt.Sprintf(`  # "traefik.http.routers.%s-tls.tls.certresolver=letsencrypt",`, appName))
	w("")
	w(`  # Force-redirect HTTP → HTTPS (uncomment along with the TLS block above):`)
	w(fmt.Sprintf(`  # "traefik.http.routers.%s.middlewares=%s-https",`, appName, appName))
	w(fmt.Sprintf(`  # "traefik.http.middlewares.%s-https.redirectscheme.scheme=https",`, appName))
	w("]")
	w("")
	w("[health_check]")
	w("# Used by nomad to gate deployments. / is a safe default, but write a real")
	w("# /healthz endpoint as soon as you can.")
	w(`path = "/"`)
	w("port = 8080")
	w(`interval = "30s"`)
	w(`timeout = "5s"`)
	w("")
	w("[hooks]")
	w("# Optional. Each command runs in a `docker run --rm` against the built image,")
	w("# with the resolved env vars injected. The pre hook runs BEFORE the rolling")
	w("# deploy starts; the post hook runs AFTER it is healthy. Both fail-fast.")
	w(`# pre  = "./scripts/migrate.sh"`)
	w(`# post = "./scripts/notify.sh"`)
	w("")
	w("# run overrides the image's CMD when nomad launches the long-lived process.")
	w("# Leave empty (or omit) to use the image's default.")
	w(`# run  = ""`)
	w("")
	w("[branches]")
	w("# Push → environment. Add lines to deploy other branches to other envs.")
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

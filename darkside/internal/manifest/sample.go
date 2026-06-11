package manifest

import (
	"fmt"
	"strings"
)

// Sample returns a copy-pasteable darkside.toml for a new app. The public key
// and key_id are pre-filled from the dashboard so the user never has to look
// them up manually.
func Sample(appName, branch, envFile, keyID, publicKey string) string {
	bt := "`"

	var b strings.Builder
	w := func(s string) { b.WriteString(s); b.WriteByte('\n') }

	w("# darkside.toml — paste at the root of your repo and commit it.")
	w("")
	w(fmt.Sprintf("name    = %q", appName))
	w(fmt.Sprintf("branch  = %q  # pushes to this branch trigger a deploy", branch))
	w(fmt.Sprintf("env_file = %q  # age-encrypted env vars", envFile))
	w(fmt.Sprintf("key_id   = %q  # rotate in the dashboard to get a new key", keyID))
	if publicKey != "" {
		w(fmt.Sprintf("# age public key (for encrypting env.age locally): %s", publicKey))
	}
	w("")
	w("[build]")
	w(`context    = "."`)
	w(`dockerfile = "Dockerfile"`)
	w("")
	w("[deploy]")
	w("count = 1")
	w("")
	w("# Traefik labels — passed through to Consul service tags.")
	w("# Traefik discovers them via the Consul catalog provider.")
	w("traefik_tags = [")
	w(`  "traefik.enable=true",`)
	w(fmt.Sprintf(`  "traefik.http.routers.%s.rule=Host(%sYOUR-HOST%s)",`, appName, bt, bt))
	w(fmt.Sprintf(`  "traefik.http.routers.%s.entrypoints=websecure",`, appName))
	w(fmt.Sprintf(`  "traefik.http.routers.%s.tls.certresolver=letsencrypt",`, appName))
	w(fmt.Sprintf(`  "traefik.http.services.%s.loadbalancer.server.port=8080",`, appName))
	w("]")
	w("")
	w("[health_check]")
	w(`path     = "/"`)
	w("port     = 8080")
	w(`interval = "30s"`)
	w(`timeout  = "5s"`)
	w("")
	w("[hooks]")
	w("# pre runs before the rolling deploy; post runs after it is healthy.")
	w(`# pre  = "./scripts/migrate.sh"`)
	w(`# post = "./scripts/notify.sh"`)
	w(`# run  = ""   # override the image's CMD`)

	return b.String()
}

// Package playbooks exposes the Ansible playbooks darkside-fleet runs against
// cluster nodes, embedded into the binary so the tool ships as a single
// executable. The runner extracts each playbook to a temp file at invocation
// time; the PlaybookService UI handler reads directly from this FS.
package playbooks

import "embed"

//go:embed *.yml
var FS embed.FS

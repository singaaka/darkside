package ansible

import (
	"fmt"
	"strings"

	"github.com/singaaka/darkside-node-manager/internal/db/dbgen"
)

// BuildInventory generates an ini-format Ansible inventory for a set of nodes.
// Each node becomes a host entry under the [cluster] group.
func BuildInventory(nodes []dbgen.Node, sshKeyPath string) string {
	var b strings.Builder
	b.WriteString("[cluster]\n")
	for _, n := range nodes {
		key := sshKeyPath
		if n.SshKeyPath != "" {
			key = n.SshKeyPath
		}
		b.WriteString(fmt.Sprintf(
			"%s ansible_host=%s ansible_user=%s ansible_ssh_private_key_file=%s ansible_ssh_common_args='-o StrictHostKeyChecking=no'\n",
			n.Name, n.PublicIP, n.SshUser, key,
		))
	}
	b.WriteString("\n[cluster:vars]\n")
	b.WriteString("ansible_python_interpreter=/usr/bin/python3\n")
	return b.String()
}

// BuildSingleHostInventory builds an inventory containing only one node.
func BuildSingleHostInventory(n dbgen.Node) string {
	return BuildInventory([]dbgen.Node{n}, n.SshKeyPath)
}

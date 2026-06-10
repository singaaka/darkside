// Package deployer runs the deploy half of a deployment: decrypt the env file,
// run the pre hook, render + submit the nomad job, watch it become healthy,
// then run the post hook. It picks up after builder leaves the deployment at
// status "built".
package deployer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/singaaka/darkside/internal/ageenv"
	"github.com/singaaka/darkside/internal/db/dbgen"
	"github.com/singaaka/darkside/internal/manifest"
)

// resolveEnv reads the encrypted env file from the cloned repo, decrypts it
// with the environment's age private key, and returns the resolved env map +
// its JSON snapshot for persistence on the deployment row.
func resolveEnv(workDir string, m *manifest.Manifest, env dbgen.Environment) (map[string]string, string, error) {
	envBlock := m.EnvironmentByName(env.Name)
	if envBlock == nil {
		return nil, "", fmt.Errorf("manifest has no [[environments]] block named %q", env.Name)
	}
	abs := filepath.Join(workDir, envBlock.EnvFile)
	cipher, err := os.ReadFile(abs)
	if err != nil {
		return nil, "", fmt.Errorf("read %s: %w", envBlock.EnvFile, err)
	}
	plain, err := ageenv.Decrypt(cipher, env.AgePrivateKey)
	if err != nil {
		return nil, "", fmt.Errorf("decrypt %s: %w", envBlock.EnvFile, err)
	}
	resolved, err := ageenv.ParseEnvFile(plain)
	if err != nil {
		return nil, "", fmt.Errorf("parse %s: %w", envBlock.EnvFile, err)
	}
	return resolved, snapshotJSON(resolved), nil
}

// snapshotJSON returns a deterministic JSON of the env map for storage. We do
// NOT redact values — operators asked for full visibility at this stage.
func snapshotJSON(env map[string]string) string {
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make(map[string]string, len(env))
	for _, k := range keys {
		out[k] = env[k]
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	return string(b)
}

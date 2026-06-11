package builder

// githubfiles.go — helpers for fetching files from a GitHub repo at a specific commit.

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/singaaka/darkside-paas/internal/ageenv"
	"github.com/singaaka/darkside-paas/internal/manifest"
)

// fetchManifest downloads darkside.toml from the repo at the given commit and parses it.
func fetchManifest(ctx context.Context, repoFullName, sha, token string) (*manifest.Manifest, error) {
	content, err := fetchFileContent(ctx, repoFullName, sha, "darkside.toml", token)
	if err != nil {
		return nil, fmt.Errorf("fetch darkside.toml: %w", err)
	}
	return manifest.Parse(content)
}

// decryptEnvFromRepo downloads the age-encrypted env file from the repo and decrypts it.
func decryptEnvFromRepo(ctx context.Context, repoFullName, sha, envFile, privateKey, token string) (map[string]string, error) {
	content, err := fetchFileContent(ctx, repoFullName, sha, envFile, token)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", envFile, err)
	}
	plain, err := ageenv.Decrypt(content, privateKey)
	if err != nil {
		return nil, fmt.Errorf("decrypt %s: %w", envFile, err)
	}
	return ageenv.ParseEnvFile(plain)
}

// fetchFileContent fetches a file's raw content from the GitHub Contents API.
func fetchFileContent(ctx context.Context, repoFullName, ref, path, token string) ([]byte, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/contents/%s?ref=%s", repoFullName, path, ref)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%s not found in repo at %s", path, ref)
	}
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github contents api: %d %s", resp.StatusCode, string(body))
	}
	var out struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	if out.Encoding != "base64" {
		return nil, fmt.Errorf("unexpected encoding: %s", out.Encoding)
	}
	// GitHub wraps base64 in newlines.
	decoded := make([]byte, base64.StdEncoding.DecodedLen(len(out.Content)))
	n, err := base64.StdEncoding.Decode(decoded, []byte(out.Content))
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}
	return decoded[:n], nil
}

package builder

// registry.go — helpers for checking the private Docker registry.

import (
	"context"
	"fmt"
	"net/http"
)

// imageExistsInRegistry checks whether a given image tag already exists in the
// private registry. Used for rollback to skip the build step.
func imageExistsInRegistry(ctx context.Context, registryAddr, appName, tag string) (bool, error) {
	url := fmt.Sprintf("http://%s/v2/%s/manifests/%s", registryAddr, appName, tag)
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}
	_ = resp.Body.Close()
	return resp.StatusCode == http.StatusOK, nil
}

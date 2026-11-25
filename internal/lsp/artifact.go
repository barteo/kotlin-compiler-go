package lsp

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const centralRepository = "https://repo.maven.apache.org/maven2"

var artifactClient = &http.Client{Timeout: 45 * time.Second}

// download caches only complete responses. Separate processes may both
// fetch a cache miss, but readers always see a complete POM or JAR.
func (r *resolver) download(rel string) (string, error) {
	if r.cache == "" {
		return "", fmt.Errorf("cannot locate the kc-lsp artifact cache")
	}
	if r.remote == "" {
		return "", fmt.Errorf("missing cached artifact %s", rel)
	}
	// Repositories can publish different bytes under the same coordinates.
	repository := sha256.Sum256([]byte(strings.TrimRight(r.remote, "/")))
	path := filepath.Join(r.cache, fmt.Sprintf("%x", repository[:8]), filepath.FromSlash(rel))
	if exists(path) {
		return path, nil
	}
	if r.offline {
		return "", fmt.Errorf("missing cached artifact %s (offline)", rel)
	}
	u, err := url.Parse(r.remote)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return "", fmt.Errorf("invalid HTTPS Maven repository URL")
	}
	u = u.JoinPath(strings.Split(rel, "/")...)
	req, err := http.NewRequestWithContext(r.ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}
	res, err := r.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", rel, err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("resolve %s: %s", rel, res.Status)
	}
	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".kc-lsp-download-")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())
	_, copyErr := io.Copy(f, res.Body)
	err = f.Close()
	if copyErr != nil {
		return "", copyErr
	}
	if err != nil {
		return "", err
	}
	if err = r.ctx.Err(); err != nil {
		return "", err
	}
	if err = os.Rename(f.Name(), path); err != nil && !exists(path) {
		return "", err
	}
	return path, nil
}

package lsp

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

type truncated struct{ io.Reader }

func (t truncated) Read(p []byte) (int, error) {
	n, err := t.Reader.Read(p)
	if err == io.EOF {
		err = io.ErrUnexpectedEOF
	}
	return n, err
}

func TestNativeArtifactAcquisition(t *testing.T) {
	t.Setenv("MAVEN_REPO_LOCAL", t.TempDir())
	t.Setenv("GRADLE_USER_HOME", t.TempDir())
	t.Setenv("KC_LSP_CACHE_DIR", t.TempDir())
	t.Setenv("KC_LSP_MAVEN_REPOSITORY", "https://artifacts.example.test/repository")
	t.Setenv("KC_LSP_OFFLINE", "0")
	r := newResolver(context.Background())
	attempts := 0
	r.client = &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/repository/org/sample/api/1/api-1.jar" {
			t.Fatal(req.URL)
		}
		attempts++
		var body io.Reader = strings.NewReader("complete contents")
		if attempts == 1 {
			body = truncated{strings.NewReader("partial")}
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(body), Header: http.Header{}}, nil
	})}
	c := coordinate{"org.sample", "api", "1"}
	if _, err := r.artifact(c, "jar"); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("truncated response %v", err)
	}
	filepath.Walk(r.cache, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			t.Fatalf("incomplete cache entry %s", path)
		}
		return nil
	})
	path, err := r.artifact(c, "jar")
	if err != nil || string(input(t, path)) != "complete contents" {
		t.Fatal(path, err)
	}
	r.offline = true
	if got, err := r.artifact(c, "jar"); err != nil || got != path || attempts != 2 {
		t.Fatal(got, err, attempts)
	}
	r.remote = "https://other.example.test"
	if _, err := r.artifact(c, "jar"); err == nil {
		t.Fatal("different mirror reused cached bytes")
	}
}

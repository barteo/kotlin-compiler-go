package classpath

import (
	"archive/zip"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type jar struct{ r *zip.ReadCloser }

func (j *jar) read(e *entry) ([]byte, error) {
	rc, err := e.file.(*zip.File).Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func (j *jar) close() error { return j.r.Close() }

// addJar indexes a jar. Opening it reads only the central directory.
func (p *Path) addJar(path string, friend bool) error {
	r, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	j := &jar{r: r}
	p.sources = append(p.sources, j)
	for _, f := range r.File {
		if isResource(f.Name) {
			p.addResource(f.Name, entry{src: j, file: f})
			continue
		}
		name, ok := strings.CutSuffix(f.Name, ".class")
		// Multi-release variants under META-INF/versions are ignored.
		if !ok || strings.HasPrefix(name, "META-INF/") || strings.HasSuffix(name, "module-info") {
			continue
		}
		p.add(name, entry{src: j, file: f, friend: friend})
	}
	return nil
}

type dir struct{}

func (dir) read(e *entry) ([]byte, error) { return os.ReadFile(e.file.(string)) }
func (dir) close() error                  { return nil }

func (p *Path) addDir(root string, friend bool) error {
	src := dir{}
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel := filepath.ToSlash(rel); isResource(rel) {
			p.addResource(rel, entry{src: src, file: path})
			return nil
		}
		if !strings.HasSuffix(path, ".class") {
			return nil
		}
		p.add(filepath.ToSlash(strings.TrimSuffix(rel, ".class")), entry{src: src, file: path, friend: friend})
		return nil
	})
}

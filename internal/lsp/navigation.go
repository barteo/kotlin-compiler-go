package lsp

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf16"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/analysis/ka"
	"github.com/barteo/kotlin-compiler-go/internal/javasrc"
	"github.com/barteo/kotlin-compiler-go/psi"
)

func hierarchyItem(n psi.NodeRef) object {
	o := symbol(n)
	o["uri"] = uri(n.ContainingFile().Path())
	return o
}
func (s *server) hierarchy(ctx context.Context, method string, i *index, target psi.NodeRef) (any, error) {
	out := []object{}
	if !target.Valid() {
		return out, nil
	}
	d := target.Parent()
	switch method {
	case "textDocument/prepareTypeHierarchy":
		if d.NumericKind() == psi.KindCLASS || d.NumericKind() == psi.KindOBJECT_DECLARATION {
			return []object{hierarchyItem(d)}, nil
		}
		return out, nil
	case "textDocument/prepareCallHierarchy":
		if d.NumericKind() == psi.KindFUN || d.NumericKind() == psi.KindPROPERTY {
			return []object{hierarchyItem(d)}, nil
		}
		return out, nil
	}
	all, err := s.relevant(ctx, target)
	if err != nil {
		return nil, err
	}
	seen := map[psi.NodeRef]bool{}
	calls := map[psi.NodeRef][]span{}
	callSites := map[psi.NodeRef]bool{}
	for _, idx := range all {
		for _, decl := range idx.decls {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			name := decl.NameIdentifier()
			sym := idx.symbols[name]
			if method == "textDocument/implementation" {
				if slices.Contains(sym.Supertypes, i.symbols[target].QualifiedName) || slices.ContainsFunc(sym.Overrides, func(id analysis.ElementID) bool { return idx.node(&id).NameIdentifier() == target }) {
					seen[decl] = true
				}
			}
			if method == "typeHierarchy/subtypes" || method == "typeHierarchy/supertypes" {
				supers := decl.ChildKind(psi.KindSUPER_TYPE_LIST)
				for entry := supers.FirstChild(); entry.Valid(); entry = entry.NextSibling() {
					base := factName(entry.ChildKind(psi.KindTYPE_REFERENCE))
					parent := idx.targets[base]
					if !parent.Valid() {
						continue
					}
					if method == "typeHierarchy/subtypes" && parent == target {
						seen[decl] = true
					}
					if method == "typeHierarchy/supertypes" && name == target {
						seen[parent.Parent()] = true
					}
				}
			}
		}
		if strings.HasPrefix(method, "callHierarchy/") {
			for ref, t := range idx.targets {
				if ref == t || callSites[ref] {
					continue
				}
				c := ancestor(ref, psi.KindCALL_EXPRESSION)
				if !c.Valid() || factName(c) != ref {
					continue
				}
				owner := ancestor(c.Parent(), psi.KindFUN, psi.KindPROPERTY)
				if !owner.Valid() {
					continue
				}
				if method == "callHierarchy/incomingCalls" && t == target {
					calls[owner] = append(calls[owner], rng(ref))
					callSites[ref] = true
					seen[owner] = true
				}
				if method == "callHierarchy/outgoingCalls" && owner == d {
					callee := t.Parent()
					calls[callee] = append(calls[callee], rng(ref))
					callSites[ref] = true
					seen[callee] = true
				}
			}
		}
	}
	nodes := slices.Collect(maps.Keys(seen))
	slices.SortFunc(nodes, func(a, b psi.NodeRef) int {
		if v := strings.Compare(a.ContainingFile().Path(), b.ContainingFile().Path()); v != 0 {
			return v
		}
		return int(a.ID()) - int(b.ID())
	})
	for _, n := range nodes {
		h := hierarchyItem(n)
		switch method {
		case "textDocument/implementation":
			out = append(out, object{"uri": h["uri"], "range": h["selectionRange"]})
		case "callHierarchy/incomingCalls", "callHierarchy/outgoingCalls":
			key := "from"
			if method == "callHierarchy/outgoingCalls" {
				key = "to"
			}
			rs := calls[n]
			slices.SortFunc(rs, func(a, b span) int {
				if before(a.Start, b.Start) {
					return -1
				}
				if before(b.Start, a.Start) {
					return 1
				}
				return 0
			})
			out = append(out, object{key: h, "fromRanges": rs})
		default:
			out = append(out, h)
		}
	}
	return out, nil
}
func javaRanges(model *ka.EditorModel, text string, sym analysis.Symbol, call *analysis.Call) []span {
	f, _ := javasrc.Parse("source.java", text)
	if f == nil {
		return nil
	}
	class := sym.Owner
	if class == "" && sym.Kind == "class" {
		class = sym.Name
	}
	var hits []span
	add := func(pos javasrc.NamePosition, name string) {
		lines := strings.Split(text, "\n")
		if pos.Line < 1 || pos.Line > len(lines) || pos.Column < 1 {
			return
		}
		l := lines[pos.Line-1]
		off := pos.Column - 1
		if off+len(name) > len(l) || l[off:off+len(name)] != name {
			return
		}
		c := len(utf16.Encode([]rune(l[:off])))
		hits = append(hits, span{position{pos.Line - 1, c}, position{pos.Line - 1, c + len(utf16.Encode([]rune(name)))}})
	}
	var visit func(*javasrc.Class, string)
	visit = func(c *javasrc.Class, owner string) {
		id := owner + c.Name
		if id == class {
			if sym.Kind == "class" {
				add(c.NamePosition, c.Name)
			}
			for _, m := range c.Methods {
				if m.Name == sym.Name && (call == nil || len(call.Parameters) == len(m.Params)) {
					if call != nil && call.JVMDescriptor != "" && model.JavaMethodDescriptor(c, m) != call.JVMDescriptor {
						continue
					}
					name := m.Name
					if name == "<init>" {
						name = c.Name
					}
					add(m.NamePosition, name)
				}
			}
			for _, f := range c.Fields {
				if f.Name == sym.Name && (call == nil || sym.Kind == "property") {
					add(f.NamePosition, f.Name)
				}
			}
		}
		for _, n := range c.Nested {
			visit(n, id+"$")
		}
	}
	prefix := ""
	if f.Package != "" {
		prefix = strings.ReplaceAll(f.Package, ".", "/") + "/"
	}
	for _, c := range f.Classes {
		visit(c, prefix)
	}
	return hits
}

// kotlinTargets finds declarations of the selected name before overload
// resolution. Top-level overloads can live in different archive entries.
func kotlinTargets(s *psi.Snapshot, sym analysis.Symbol, c *analysis.Call) []psi.NodeRef {
	pkg := s.RootNode().ChildKind(psi.KindPACKAGE_DIRECTIVE).Path()
	var hits []psi.NodeRef
	psi.WalkNodes(s.RootNode(), func(n psi.NodeRef) bool {
		if !n.Declaration().Valid() || !n.NameIdentifier().Valid() {
			return true
		}
		switch sym.Kind {
		case "class", "constructor":
			if !slices.Contains([]psi.Kind{psi.KindCLASS, psi.KindOBJECT_DECLARATION, psi.KindTYPEALIAS}, n.NumericKind()) {
				return n.NumericKind() != psi.KindFUN
			}
		case "function":
			if n.NumericKind() != psi.KindFUN {
				return true
			}
		case "property":
			if n.NumericKind() != psi.KindPROPERTY && n.NumericKind() != psi.KindVALUE_PARAMETER {
				return true
			}
		}
		name := n.Name()
		for parent := n.Parent(); parent.Valid(); parent = parent.Parent() {
			if parent.NumericKind() == psi.KindCLASS || parent.NumericKind() == psi.KindOBJECT_DECLARATION {
				name = parent.Name() + "." + name
			}
		}
		if pkg != "" {
			name = pkg + "." + name
		}
		if name == sym.QualifiedName && (c == nil || n.NumericKind() != psi.KindFUN ||
			len(n.Parameters()) == len(c.Parameters) && n.ReceiverType().Valid() == c.Extension) {
			hits = append(hits, n)
		}
		return n.NumericKind() != psi.KindFUN
	})
	return hits
}

func attachedOverload(ctx context.Context, session *ka.Session, hits []psi.NodeRef, c *analysis.Call) psi.NodeRef {
	if len(hits) == 1 && (c == nil || hits[0].NumericKind() != psi.KindFUN ||
		len(c.Parameters) == 0 && !c.Extension) {
		return hits[0]
	}
	if len(hits) == 0 || c == nil {
		return psi.NodeRef{}
	}
	// Resolve imported aliases, nested types and receivers in the source
	// declaration's own scope, using the consumer's compile classpath.
	var snaps []*psi.Snapshot
	for _, n := range hits {
		if !slices.Contains(snaps, n.ContainingFile()) {
			snaps = append(snaps, n.ContainingFile())
		}
	}
	model, err := session.CheckEditor(ctx, snaps, nil)
	if err != nil {
		return psi.NodeRef{}
	}
	defer model.Close()
	types := make(map[analysis.ElementID]*analysis.Type)
	for _, fact := range model.Facts.Facts {
		if fact.Symbol != 0 && fact.Role == "declaration" {
			types[fact.Element] = &model.Facts.Symbols[fact.Symbol-1].Type
		} else if fact.Type != nil {
			types[fact.Element] = fact.Type
		}
	}
	resolved := func(ref psi.NodeRef) *analysis.Type {
		if !ref.Valid() {
			return nil
		}
		return types[analysis.ElementID{Snapshot: ref.ContainingFile().ID(), Node: ref.ID()}]
	}
	var found psi.NodeRef
	for _, n := range hits {
		if n.NumericKind() != psi.KindFUN || !sameType(resolved(n.ReceiverType()), c.DeclaredReceiver) {
			continue
		}
		matches := true
		for j, p := range n.Parameters() {
			t := resolved(p)
			if !sameType(t, &c.Parameters[j]) {
				matches = false
				break
			}
		}
		if matches {
			if found.Valid() {
				return psi.NodeRef{}
			}
			found = n
		}
	}
	return found
}

// Extract one matching source under a private temporary directory. The URI
// is an ordinary file URI that even clients without a jar: provider can open.
func libraryDefinition(ctx context.Context, w *workspace, path string, i *index, n psi.NodeRef) *location {
	sym := i.symbols[n]
	if sym.Kind == "" {
		return nil
	}
	c := i.facts[n.Parent()].Call
	if call := ancestor(n, psi.KindCALL_EXPRESSION); call.Valid() && factName(call) == n {
		c = i.facts[call].Call
	}
	return resolveLibrary(ctx, w, path, i, sym, c)
}
func resolveLibrary(ctx context.Context, w *workspace, path string, i *index, sym analysis.Symbol, c *analysis.Call) *location {
	m := w.module(path)
	group := w.p.group(m)
	var java []location
	for p, t := range w.java {
		if slices.Contains(group, w.p.owner(p)) {
			for _, r := range javaRanges(i.model, t, sym, c) {
				java = append(java, location{uri(p), r})
			}
		}
	}
	if len(java) > 1 {
		return nil
	}
	if len(java) == 1 {
		return &java[0]
	}
	var sources []string
	for _, dep := range group {
		sources = append(sources, dep.Sources...)
	}
	cfg := w.p.config(m)
	if cfg.JDKHome != "" {
		sources = append(sources, filepath.Join(cfg.JDKHome, "lib", "src.zip"))
	}
	var hits []psi.NodeRef
	seen := map[string]bool{}
	for _, src := range sources {
		if seen[src] {
			continue
		}
		seen[src] = true
		locations, candidates := inArchive(ctx, w, i.model, src, sym, c)
		for _, loc := range locations {
			if !slices.Contains(java, loc) {
				java = append(java, loc)
			}
		}
		hits = append(hits, candidates...)
	}
	if len(java) > 1 || ctx.Err() != nil {
		return nil
	}
	if len(java) == 1 {
		return &java[0]
	}
	n := attachedOverload(ctx, w.sessions[m], hits, c)
	if !n.Valid() {
		return nil
	}
	snap := n.ContainingFile()
	p, err := w.materialize(snap.Path(), []byte(snap.Source()))
	if err != nil {
		return nil
	}
	return &location{uri(p), rng(n.NameIdentifier())}
}
func inArchive(ctx context.Context, w *workspace, model *ka.EditorModel, src string, sym analysis.Symbol, c *analysis.Call) ([]location, []psi.NodeRef) {
	z, err := zip.OpenReader(src)
	if err != nil {
		return nil, nil
	}
	defer z.Close()
	owner := sym.Owner
	if owner == "" && sym.Kind == "class" {
		owner = sym.Name
	}
	outer, _, _ := strings.Cut(owner, "$")
	var hits []psi.NodeRef
	var javaLocations []location
	for _, f := range z.File {
		if ctx.Err() != nil {
			return nil, nil
		}
		java := strings.HasSuffix(f.Name, ".java")
		if java && (outer == "" || !strings.HasSuffix("/"+f.Name, "/"+outer+".java")) || !java && !strings.HasSuffix(f.Name, ".kt") || f.UncompressedSize64 > 8<<20 {
			continue
		}
		b, err := readSource(f)
		if err != nil {
			continue
		}
		if !java {
			if bytes.Contains(b, []byte(short(sym.QualifiedName))) {
				snap := psi.ParseDocument(f.Name, string(b))
				hits = append(hits, kotlinTargets(snap, sym, c)...)
			}
			continue
		}
		rs := javaRanges(model, string(b), sym, c)
		if len(rs) > 0 {
			if p, err := w.materialize(f.Name, b); err == nil {
				for _, r := range rs {
					javaLocations = append(javaLocations, location{uri(p), r})
				}
			}
		}
	}
	return javaLocations, hits
}

// librarySymbols searches source attachments on demand. Keep their parsed
// Kotlin trees for subsequent queries in the same imported workspace.
func (w *workspace) librarySymbols(ctx context.Context, query string) ([]object, error) {
	out := []object{}
	if query == "" {
		return out, nil
	}
	seen := map[string]bool{}
	for _, m := range w.p.Modules {
		for _, src := range m.Sources {
			if seen[src] {
				continue
			}
			seen[src] = true
			if w.attached == nil {
				w.attached = map[string][]*psi.Snapshot{}
			}
			snaps, ok := w.attached[src]
			if !ok {
				z, err := zip.OpenReader(src)
				if err != nil {
					return nil, err
				}
				snaps, err = w.indexSources(ctx, z)
				z.Close()
				if err != nil {
					return nil, err
				}
				w.attached[src] = snaps
			}
			for _, snap := range snaps {
				psi.WalkNodes(snap.RootNode(), func(n psi.NodeRef) bool {
					if n.Declaration().Valid() && n.NameIdentifier().Valid() && strings.Contains(strings.ToLower(n.Name()), strings.ToLower(query)) {
						out = append(out, object{"name": n.Name(), "kind": symbolKind(n), "location": loc(n.NameIdentifier())})
					}
					return n.NumericKind() != psi.KindBLOCK && n.NumericKind() != psi.KindVALUE_PARAMETER_LIST
				})
			}
		}
	}
	return out, nil
}
func (w *workspace) indexSources(ctx context.Context, z *zip.ReadCloser) ([]*psi.Snapshot, error) {
	var snaps []*psi.Snapshot
	for _, f := range z.File {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !strings.HasSuffix(f.Name, ".kt") || f.UncompressedSize64 > 8<<20 {
			continue
		}
		b, err := readSource(f)
		if err != nil {
			return nil, err
		}
		p, err := w.materialize(f.Name, b)
		if err != nil {
			return nil, err
		}
		snaps = append(snaps, psi.ParseDocument(p, string(b)))
	}
	return snaps, nil
}
func readSource(f *zip.File) ([]byte, error) {
	rd, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rd.Close()
	b, err := io.ReadAll(io.LimitReader(rd, 8<<20+1))
	if len(b) > 8<<20 {
		return nil, fmt.Errorf("source exceeds 8 MiB")
	}
	return b, err
}
func (w *workspace) materialize(name string, b []byte) (string, error) {
	if w.sourceDir == "" {
		dir, err := os.MkdirTemp("", "kc-lsp-sources-")
		if err != nil {
			return "", err
		}
		w.sourceDir = dir
	}
	dir := filepath.Join(w.sourceDir, fmt.Sprintf("%x", sha256.Sum256(b)))
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	p := filepath.Join(dir, filepath.Base(name))
	if !exists(p) {
		if err := os.WriteFile(p, b, 0400); err != nil {
			return "", err
		}
	}
	return p, nil
}

package lsp

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/analysis/ka"
	"github.com/barteo/kotlin-compiler-go/psi"
)

type candidate struct {
	name, classID, importPath string
	source                    psi.NodeRef
	binary                    *ka.Decl
	kind, rank                int
}

func (c candidate) detail() string {
	if c.source.Valid() {
		return summary(c.source)
	}
	if c.binary != nil {
		return librarySignature(c.binary)
	}
	return strings.ReplaceAll(c.classID, "/", ".")
}

// A nil receiver denotes unqualified lookup. A non-nil unknown receiver is
// an incomplete selector and must never fall back to unrelated lexical names.
func (i *index) receivers(at psi.NodeRef, explicit *analysis.Type) []analysis.Type {
	if explicit != nil {
		return []analysis.Type{*explicit}
	}
	var out []analysis.Type
	for n := at; n.Valid(); n = n.Parent() {
		var typ *analysis.Type
		switch n.NumericKind() {
		case psi.KindLAMBDA_EXPRESSION, psi.KindFUNCTION_LITERAL:
			typ = i.facts[n].LambdaReceiver
		case psi.KindFUN:
			typ = i.facts[n.ReceiverType()].Type
		case psi.KindCLASS, psi.KindOBJECT_DECLARATION:
			sym := i.symbols[n.NameIdentifier()]
			typ = &sym.Type
			if typ.Name != "" {
				out = append(out, *typ)
			}
			if n.Parent().NumericKind() == psi.KindCLASS_BODY && !n.ChildKind(psi.KindMODIFIER_LIST).ChildKind(psi.KindINNER_KEYWORD).Valid() {
				return out
			}
			continue
		}
		if typ != nil && typ.Name != "" {
			out = append(out, *typ)
		}
	}
	return out
}
func dottedName(name string) string {
	return strings.ReplaceAll(strings.ReplaceAll(name, "/", "."), "$", ".")
}

// Constructor val/var parameters declare members visible through an instance.
func constructorProperty(d psi.NodeRef) bool {
	return d.NumericKind() == psi.KindVALUE_PARAMETER && d.Parent().Parent().NumericKind() == psi.KindPRIMARY_CONSTRUCTOR && (d.ChildKind(psi.KindVAL_KEYWORD).Valid() || d.ChildKind(psi.KindVAR_KEYWORD).Valid())
}

func acceptsReceiver(receivers []analysis.Type, name string) bool {
	if name == "" {
		return false
	}
	name = dottedName(name)
	for _, recv := range receivers {
		if recv.Name == name || slices.Contains(recv.Supertypes, name) {
			return true
		}
	}
	return false
}

type fileImports struct {
	packages map[string]bool   // true for star imports, false for individual imports only
	aliases  map[string]string // fully qualified name to its spelling in this file
	names    map[string]bool
}

func importsAt(at psi.NodeRef) fileImports {
	root := at.ContainingFile().RootNode()
	pkg := root.ChildKind(psi.KindPACKAGE_DIRECTIVE).Path()
	packages := map[string]bool{"kotlin": true, "kotlin.collections": true, "kotlin.io": true, "kotlin.ranges": true, "kotlin.sequences": true, "kotlin.text": true, pkg: true}
	imports := map[string]string{}
	localNames := map[string]bool{}
	for n := root.ChildKind(psi.KindIMPORT_LIST).FirstChild(); n.Valid(); n = n.NextSibling() {
		if !n.Import().Valid() {
			continue
		}
		if n.IsAllUnder() {
			packages[n.Path()] = true
			continue
		}
		name := n.Alias()
		if name == "" {
			name = short(n.Path())
		}
		imports[n.Path()] = name
		localNames[name] = true
		if pos := strings.LastIndexByte(n.Path(), '.'); pos >= 0 {
			p := n.Path()[:pos]
			if !packages[p] {
				packages[p] = false
			}
		}
	}
	return fileImports{packages, imports, localNames}
}

func (i *index) candidates(ctx context.Context, at psi.NodeRef, prefix string, explicit *analysis.Type, allowImports bool) []candidate {
	receivers := i.receivers(at, explicit)
	scope := importsAt(at)
	source := i.sourceCandidates(at, prefix, explicit, allowImports, receivers, scope)
	return append(source, i.libraryCandidates(ctx, prefix, explicit, receivers, scope)...)

}

func (i *index) sourceCandidates(at psi.NodeRef, prefix string, explicit *analysis.Type, allowImports bool, receivers []analysis.Type, scope fileImports) []candidate {
	var out []candidate
	locals := map[string]int{}
	decls := i.decls
	if explicit != nil {
		decls = i.byReceiver[explicit.Name]
		for _, super := range explicit.Supertypes {
			if more := i.byReceiver[super]; len(more) > 0 {
				decls = slices.Concat(decls, more)
			}
		}
	}
	for _, d := range decls {
		name := d.Name()
		needsImport := ""
		top := d.Parent().NumericKind() == psi.KindFILE
		if top {
			path := d.ContainingFile().RootNode().ChildKind(psi.KindPACKAGE_DIRECTIVE).Path()
			fq := name
			if path != "" {
				fq = path + "." + name
			}
			if alias := scope.aliases[fq]; alias != "" {
				name = alias
			} else if !scope.packages[path] {
				if !allowImports || prefix == "" || path == "" || scope.names[name] {
					continue
				}
				needsImport = fq
			}
		}
		if !strings.HasPrefix(name, prefix) || !visibleAt(d, at) || !i.moduleVisible(d, at) {
			continue
		}
		sym := i.symbols[d.NameIdentifier()]
		rank := 2
		member := d.Parent().NumericKind() == psi.KindCLASS_BODY
		memberOwner := sym.Owner
		if constructorProperty(d) {
			member = true
			memberOwner = i.symbols[d.Parent().Parent().Parent().NameIdentifier()].QualifiedName
		}
		if rt := d.ReceiverType(); rt.Valid() {
			t := i.facts[rt].Type
			if t == nil || !acceptsReceiver(receivers, t.Name) {
				continue
			}
			if member && !under(at, d.Parent().Parent()) {
				continue
			}
		} else if member {
			if !acceptsReceiver(receivers, memberOwner) && !(explicit == nil && under(at, d.Parent().Parent())) {
				continue
			}
			rank = 1
		} else if explicit != nil {
			continue
		}
		if !member && !top {
			rank = 0
		}
		if needsImport != "" {
			rank = 4
		}
		kind := 6
		switch symbolKind(d) {
		case 5, 11:
			kind = 7
		case 6:
			kind = 2
		case 12:
			kind = 3
		case 7:
			kind = 10
		}
		c := candidate{name: name, source: d, kind: kind, rank: rank, importPath: needsImport}
		if rank == 0 && (sym.Kind == "parameter" || sym.Kind == "local") {
			// Kotlin resolves a local name to the declaration in the nearest
			// enclosing scope. Both declaration nodes contain the same cursor
			// in their scope, so the inner declaration follows the outer one.
			if j, ok := locals[name]; ok {
				if d.Range().Start > out[j].source.Range().Start {
					out[j] = c
				}
				continue
			}
			locals[name] = len(out)
		}
		out = append(out, c)
	}
	return out
}

func (i *index) libraryCandidates(ctx context.Context, prefix string, explicit *analysis.Type, receivers []analysis.Type, scope fileImports) []candidate {
	var out []candidate
	seen := map[*ka.Decl]bool{}
	for _, receiver := range receivers {
		for _, name := range append([]string{receiver.Name}, receiver.Supertypes...) {
			if name == "" {
				continue
			}
			cl := i.model.Class(strings.ReplaceAll(name, ".", "/"))
			if cl == nil {
				continue
			}
			for _, d := range cl.Members {
				if seen[d] || d.Visibility != ka.Public || !strings.HasPrefix(d.Name, prefix) || d.Kind != ka.KindFunction && d.Kind != ka.KindProperty {
					continue
				}
				seen[d] = true
				kind := 2
				if d.Kind == ka.KindProperty {
					kind = 10
				}
				out = append(out, candidate{name: d.Name, binary: d, kind: kind, rank: 1})
			}
		}
	}
	for p, star := range scope.packages {
		if ctx.Err() != nil {
			break
		}
		lib := i.model.Package(p)
		if lib == nil {
			continue
		}
		for _, d := range lib.Decls {
			name := scope.aliases[p+"."+d.Name]
			if name == "" {
				if !star {
					continue
				}
				name = d.Name
			}
			if !strings.HasPrefix(name, prefix) || d.Visibility != ka.Public {
				continue
			}
			if d.Receiver != nil {
				if !acceptsReceiver(receivers, d.Receiver.ClassID) {
					continue
				}
			} else if explicit != nil {
				continue
			}
			kind := 3
			if d.Kind == ka.KindProperty {
				kind = 10
			}
			out = append(out, candidate{name: name, binary: d, kind: kind, rank: 2})
		}
		if explicit == nil {
			for _, id := range lib.Classes {
				name := short(id)
				alias := scope.aliases[p+"."+name]
				if alias != "" {
					name = alias
				} else if !star {
					continue
				}
				if strings.HasPrefix(name, prefix) {
					if cl := i.model.Class(id); cl == nil || cl.Visibility != ka.Public {
						continue
					}
					out = append(out, candidate{name: name, classID: id, kind: 7, rank: 2})
				}
			}
		}
	}
	return out
}

var escapeSnippet = strings.NewReplacer("\\", "\\\\", "$", "\\$", "}", "\\}")

type completionOptions struct {
	SnippetSupport, InsertReplaceSupport bool
	DocumentationFormat                  []string
	ResolveSupport                       struct{ Properties []string }
}
type completionData struct {
	Snapshot psi.SnapshotID `json:"snapshot,omitempty"`
	Node     psi.NodeID     `json:"node,omitempty"`
	URI      string         `json:"uri,omitempty"`
}

func (i *index) completion(ctx context.Context, s *psi.Snapshot, off int, options completionOptions) []object {
	at := s.NodeAt(max(off-1, 0))
	prefix := ""
	end, replaceEnd := off, off
	if at.NumericKind() == psi.KindIDENTIFIER && off >= at.Range().Start {
		prefix = s.Source()[at.Range().Start:off]
		end = at.Range().Start
		replaceEnd = at.Range().End
	} else if n := s.NodeAt(off); n.NumericKind() == psi.KindIDENTIFIER && n.Range().Start == off {
		replaceEnd = n.Range().End
	}
	before := strings.TrimRight(s.Source()[:end], " \t")
	member := strings.HasSuffix(before, ".")
	var receiver analysis.Type
	if member {
		p := len(strings.TrimRight(strings.TrimSuffix(before[:len(before)-1], "?"), " \t"))
		if p > 0 {
			token := s.NodeAt(p - 1)
			for n := token; n.Valid() && n.Range().End == p; n = n.Parent() {
				if f := i.facts[n]; f.Type != nil {
					receiver = *f.Type
				}
			}

			if receiver.Name == "" && len(s.Diagnostics()) > 0 && ctx.Err() == nil {
				// Complete the selector in a throwaway snapshot. Resolution of the
				// receiver, including overloaded calls, then follows ordinary kc rules.
				patched := psi.ParseDocument(s.Path(), s.Source()[:end]+"kcEditorCompletion"+s.Source()[off:])
				temp, err := i.workspace.buildIndex(ctx, s.Path(), patched)
				if err == nil {
					defer temp.model.Close()
					leaf := patched.NodeAt(p - 1)
					for n := leaf; n.Valid() && n.Range().End == p; n = n.Parent() {
						if f := temp.facts[n]; f.Type != nil {
							receiver = *f.Type
						}
					}
				}
			}
		}
	}

	var recv *analysis.Type
	if member {
		recv = &receiver
	}
	out := []object{}
	insert := span{pos(s, end), pos(s, off)}
	replace := span{insert.Start, pos(s, replaceEnd)}
	tail := strings.TrimLeft(s.Source()[replaceEnd:], " \t")
	insertCall := !strings.HasSuffix(before, "::") && !strings.HasPrefix(tail, "(") && !strings.HasPrefix(tail, "{") && !ancestor(at, psi.KindIMPORT_DIRECTIVE).Valid()
	for _, c := range i.candidates(ctx, at, prefix, recv, true) {
		detail := c.detail()
		if c.importPath != "" {
			detail += " (" + c.importPath + ")"
		}
		item := object{"label": c.name, "detail": detail, "kind": c.kind, "sortText": fmt.Sprintf("%02d-%s", c.rank, c.name)}
		name := c.name
		if c.source.Valid() {
			id := c.source.NameIdentifier()
			if id.Name() == c.name {
				name = id.Text()
			}
			source := c.source.ContainingFile()
			item["data"] = completionData{source.ID(), c.source.ID(), uri(source.Path())}
		}
		text := name
		if insertCall && (c.kind == 2 || c.kind == 3) {
			text += "()"
			if options.SnippetSupport && (c.source.Valid() && len(c.source.Parameters()) > 0 || c.binary != nil && len(c.binary.Params) > 0) {
				text = escapeSnippet.Replace(name) + "($0)"
				item["insertTextFormat"] = 2
			}
		}
		if options.InsertReplaceSupport {
			item["textEdit"] = object{"insert": insert, "replace": replace, "newText": text}
		} else {
			item["textEdit"] = edit{replace, text}
		}
		if c.importPath != "" {
			item["additionalTextEdits"] = addImports(s, map[string]bool{c.importPath: true})
		}
		out = append(out, item)
	}
	slices.SortStableFunc(out, func(a, b object) int {
		return cmp.Or(strings.Compare(a["sortText"].(string), b["sortText"].(string)), strings.Compare(a["detail"].(string), b["detail"].(string)))
	})
	if slices.Contains(options.ResolveSupport.Properties, "detail") {
		for _, item := range out {
			if item["data"] != nil {
				delete(item, "detail")
			}
		}
	}
	return out
}

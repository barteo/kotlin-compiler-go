package lsp

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/barteo/kotlin-compiler-go/psi"
)

func (s *server) relevant(ctx context.Context, target psi.NodeRef) ([]*index, error) {
	m := s.w.module(target.ContainingFile().Path())
	seen := map[*module]bool{}
	var all []*index
	for _, path := range slices.Sorted(maps.Keys(s.w.docs)) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		owner := s.w.module(path)
		if seen[owner] || !slices.Contains(s.w.p.group(owner), m) {
			continue
		}
		i, err := s.w.check(ctx, path)
		if err != nil {
			return nil, err
		}
		all = append(all, i)
		seen[owner] = true
	}
	return all, nil
}
func (s *server) references(ctx context.Context, method, path string, at, target psi.NodeRef, p parameters) (any, error) {
	rename := method == "textDocument/rename"
	highlights := method == "textDocument/documentHighlight"
	if !target.Valid() {
		if rename {
			return nil, rpcError{-32803, "The symbol has no editable source declaration"}
		}
		return []location{}, nil
	}
	all, err := s.relevant(ctx, target)
	if err != nil {
		return nil, err
	}
	family := map[psi.NodeRef]bool{target: true}
	if rename && target.Parent().NumericKind() == psi.KindFUN && target.Parent().Parent().NumericKind() == psi.KindCLASS_BODY {
		// A rename started on an implementation can also affect consumers of a
		// sibling implementation that depend only on their common interface.
		all, err = s.overrideIndexes(ctx, all, target)
		if err != nil {
			return nil, err
		}
		family, err = overrideFamily(all, target)
		if err != nil {
			return nil, rpcError{-32803, err.Error()}
		}
	}
	seen := map[psi.NodeRef]bool{}
	for _, i := range all {
		for ref, t := range i.targets {
			if family[t] {
				seen[ref] = true
			}
		}
	}
	refs := slices.Collect(maps.Keys(seen))
	slices.SortFunc(refs, func(a, b psi.NodeRef) int {
		if n := strings.Compare(a.ContainingFile().Path(), b.ContainingFile().Path()); n != 0 {
			return n
		}
		return int(a.ID()) - int(b.ID())
	})
	if !rename {
		out := []object{}
		for _, r := range refs {
			if highlights {
				if r.ContainingFile().Path() == path {
					out = append(out, object{"range": rng(r)})
				}
			} else if r != target || p.Context.IncludeDeclaration {
				out = append(out, object{"uri": uri(r.ContainingFile().Path()), "range": rng(r)})
			}
		}
		return out, nil
	}
	if err := safeRename(all, seen, target, p.NewName, s.w.java); err != nil {
		return nil, rpcError{-32803, err.Error()}
	}
	changes := map[string][]edit{}
	for _, r := range refs {
		if r.Name() != target.Name() {
			continue
		}
		u := uri(r.ContainingFile().Path())
		changes[u] = append(changes[u], edit{rng(r), p.NewName})
	}
	return s.workspaceEdits(changes), nil
}
func (s *server) overrideIndexes(ctx context.Context, all []*index, target psi.NodeRef) ([]*index, error) {
	bases := map[psi.NodeRef]bool{target: true}
	seen := map[*index]bool{}
	// Every base declaration is in the checked module's dependency closure.
	for _, i := range all {
		seen[i] = true
		queue := []psi.NodeRef{target}
		for len(queue) > 0 {
			n := queue[0]
			queue = queue[1:]
			for _, id := range i.symbols[n].Overrides {
				parent := i.node(&id).NameIdentifier()
				if parent.Valid() && !bases[parent] {
					bases[parent] = true
					queue = append(queue, parent)
				}
			}
		}
	}
	for base := range bases {
		if base == target {
			continue
		}
		indices, err := s.relevant(ctx, base)
		if err != nil {
			return nil, err
		}
		for _, i := range indices {
			if !seen[i] {
				all = append(all, i)
				seen[i] = true
			}
		}
	}
	return all, nil
}
func overrideFamily(all []*index, target psi.NodeRef) (map[psi.NodeRef]bool, error) {
	edges := map[psi.NodeRef][]psi.NodeRef{}
	incomplete := map[psi.NodeRef]bool{}
	for _, i := range all {
		for _, decl := range i.decls {
			n := decl.NameIdentifier()
			if n.Name() != target.Name() {
				continue
			}
			sym := i.symbols[n]
			if sym.ExternalOverride || decl.ChildKind(psi.KindMODIFIER_LIST).ChildKind(psi.KindOVERRIDE_KEYWORD).Valid() && len(sym.Overrides) == 0 {
				incomplete[n] = true
			}
			for _, id := range sym.Overrides {
				base := i.node(&id).NameIdentifier()
				if base.Valid() {
					edges[n] = append(edges[n], base)
					edges[base] = append(edges[base], n)
				}
			}
		}
	}
	family := map[psi.NodeRef]bool{target: true}
	queue := []psi.NodeRef{target}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		if incomplete[n] || n.Parent().ChildKind(psi.KindMODIFIER_LIST).ChildKind(psi.KindOPERATOR_KEYWORD).Valid() {
			return nil, fmt.Errorf("rename requires an external or implicit operator declaration")
		}
		for _, next := range edges[n] {
			if !family[next] {
				family[next] = true
				queue = append(queue, next)
			}
		}
	}
	return family, nil
}
func safeRename(all []*index, refs map[psi.NodeRef]bool, target psi.NodeRef, name string, java map[string]string) error {
	probe := psi.ParseDocument("name.kt", "val "+name+" = 0")
	ds := probe.DeclarationNodes()
	if len(probe.Diagnostics()) != 0 || len(ds) != 1 || ds[0].NameIdentifier().Text() != name {
		return fmt.Errorf("invalid Kotlin identifier %q", name)
	}
	owner := target.Parent()
	mods := owner.ChildKind(psi.KindMODIFIER_LIST)
	if mods.ChildKind(psi.KindOPERATOR_KEYWORD).Valid() || owner.NumericKind() == psi.KindPROPERTY && (mods.ChildKind(psi.KindOVERRIDE_KEYWORD).Valid() || mods.ChildKind(psi.KindOPEN_KEYWORD).Valid() || mods.ChildKind(psi.KindABSTRACT_KEYWORD).Valid()) {
		return fmt.Errorf("rename needs implicit operator or property override coverage")
	}
	local := ancestor(owner.Parent(), psi.KindBLOCK)
	if owner.NumericKind() == psi.KindVALUE_PARAMETER {
		local = psi.NodeRef{}
	}
	if !local.Valid() && len(java) > 0 {
		return fmt.Errorf("rename needs Java reference coverage")
	}
	for _, i := range all {
		for _, s := range i.snaps {
			if local.Valid() && s != target.ContainingFile() {
				continue
			}
			if len(s.Diagnostics()) > 0 {
				return fmt.Errorf("resolve syntax errors before renaming")
			}
			for id := psi.NodeID(1); int(id) <= s.NodeCount(); id++ {
				n := s.Node(id)
				if n.NumericKind() != psi.KindIDENTIFIER || local.Valid() && !under(n, local) {
					continue
				}
				if n.Name() == ds[0].Name() && !refs[n] {
					return fmt.Errorf("%s could capture an existing name", name)
				}
				if n.Name() == target.Name() && !i.targets[n].Valid() {
					return fmt.Errorf("reference coverage is incomplete for %s", target.Name())
				}
			}
		}
	}
	return nil
}

var tokenTypes = []string{"namespace", "class", "enum", "interface", "struct", "typeParameter", "type", "parameter", "variable", "property", "enumMember", "event", "function", "method", "macro", "keyword", "modifier", "comment", "string", "number", "regexp", "operator", "decorator", "label"}
var tokenModifiers = []string{"declaration", "definition", "readonly", "static", "deprecated", "abstract", "async", "modification", "documentation", "defaultLibrary", "namedArgument"}

func tokenClass(n psi.NodeRef, i *index) (string, int) {
	k := n.NumericKind()
	if k == psi.KindKDOC {
		return "comment", 1 << 8
	}
	if !n.IsLeaf() {
		return "", 0
	}
	kind := n.Kind()
	switch {
	case k == psi.KindLABEL_IDENTIFIER || ancestor(n.Parent(), psi.KindLABEL, psi.KindLABEL_QUALIFIER).Valid() && k == psi.KindIDENTIFIER:
		return "label", 0
	case k == psi.KindIDENTIFIER:
		if ancestor(n.Parent(), psi.KindVALUE_ARGUMENT_NAME).Valid() {
			return "parameter", 1 << 10
		}
		d := i.targets[n].Parent()
		sym := i.symbols[n]
		mods := 0
		if d.Valid() && d.NameIdentifier() == n {
			mods |= 1
		}
		if d.ChildKind(psi.KindVAL_KEYWORD).Valid() || sym.Kind == "parameter" && !d.ChildKind(psi.KindVAR_KEYWORD).Valid() {
			mods |= 1 << 2
		}
		md := d.ChildKind(psi.KindMODIFIER_LIST)
		if md.ChildKind(psi.KindABSTRACT_KEYWORD).Valid() {
			mods |= 1 << 5
		}
		if md.ChildKind(psi.KindSUSPEND_KEYWORD).Valid() {
			mods |= 1 << 6
		}
		if i.facts[n.Parent()].Deprecated != nil {
			mods |= 1 << 4
		}
		if i.facts[n.Parent()].Package != "" {
			return "namespace", mods
		}
		switch d.NumericKind() {
		case psi.KindENUM_ENTRY:
			return "enumMember", mods
		case psi.KindTYPE_PARAMETER:
			return "typeParameter", mods
		case psi.KindTYPEALIAS:
			return "type", mods
		case psi.KindCLASS:
			if md.ChildKind(psi.KindENUM_KEYWORD).Valid() {
				return "enum", mods
			}
			if d.ChildKind(psi.KindINTERFACE_KEYWORD).Valid() {
				return "interface", mods
			}
		}
		switch sym.Kind {
		case "function", "constructor":
			if sym.Owner != "" {
				return "method", mods
			}
			return "function", mods
		case "class":
			return "class", mods
		case "parameter":
			return "parameter", mods
		case "property":
			return "property", mods
		}
		if ancestor(n, psi.KindPACKAGE_DIRECTIVE, psi.KindIMPORT_DIRECTIVE).Valid() {
			return "namespace", mods
		}
		return "variable", mods
	case strings.HasSuffix(kind, "_KEYWORD"):
		if n.Parent().NumericKind() == psi.KindMODIFIER_LIST {
			return "modifier", 0
		}
		return "keyword", 0
	case strings.Contains(kind, "COMMENT"):
		return "comment", 0
	case kind == "INTEGER_LITERAL" || kind == "FLOAT_LITERAL":
		return "number", 0
	case kind == "REGULAR_STRING_PART" || kind == "CHARACTER_LITERAL" || strings.Contains(kind, "QUOTE"):
		return "string", 0
	case n.Parent().NumericKind() == psi.KindOPERATION_REFERENCE:
		return "operator", 0
	}
	return "", 0
}
func tokens(s *psi.Snapshot, i *index, r span, limited bool) object {
	data := []int{}
	pl, pc := 0, 0
	psi.WalkNodes(s.RootNode(), func(n psi.NodeRef) bool {
		typ, mods := tokenClass(n, i)
		if typ == "" {
			return true
		}
		k := slices.Index(tokenTypes, typ)
		// Send a separate token for each nonempty line of a multiline token.
		for start := n.Range().Start; start < n.Range().End; {
			end := n.Range().End
			if j := strings.IndexByte(s.Source()[start:end], '\n'); j >= 0 {
				end = start + j
			}
			trim := end
			if trim > start && s.Source()[trim-1] == '\r' {
				trim--
			}
			a, b := pos(s, start), pos(s, trim)
			if trim > start && (!limited || overlaps(span{a, b}, r)) {
				dc := a.Character
				if a.Line == pl {
					dc -= pc
				}
				data = append(data, a.Line-pl, dc, b.Character-a.Character, k, mods)
				pl, pc = a.Line, a.Character
			}
			start = end + 1
		}
		return false
	})
	return object{"data": data}
}
func before(a, b position) bool {
	return a.Line < b.Line || a.Line == b.Line && a.Character < b.Character
}
func overlaps(a, b span) bool { return !before(a.End, b.Start) && !before(b.End, a.Start) }

// These defaults and option names follow the Kotlin editor extension.
type hintSettings struct {
	property, variable, returns, parameterType, arguments, compiled bool
}

func readHintSettings(options object) hintSettings {
	h := hintSettings{true, true, true, true, true, true}
	fields := map[string]*bool{
		"settings.types.property": &h.property, "settings.types.variable": &h.variable,
		"type.function.return": &h.returns, "type.function.parameter": &h.parameterType,
		"parameters": &h.arguments, "parameters.compiled": &h.compiled,
	}
	// Clients send either dotted keys or nested JSON objects.
	var visit func(string, object)
	visit = func(prefix string, o object) {
		for key, value := range o {
			name := prefix + key
			switch v := value.(type) {
			case map[string]any:
				visit(name+".", v)
			case bool:
				if suffix, ok := strings.CutPrefix(name, "jetbrains.kotlin.hints."); ok {
					if f := fields[suffix]; f != nil {
						*f = v
					}
				}
			}
		}
	}
	visit("", options)
	return h
}

func hints(s *psi.Snapshot, i *index, r span, h hintSettings) []object {
	out := []object{}
	add := func(at position, label string, kind int) {
		if before(at, r.Start) || before(r.End, at) {
			return
		}
		hint := object{"position": at, "label": label, "kind": kind}
		if kind == 2 {
			hint["paddingRight"] = true
		}
		out = append(out, hint)
	}
	psi.WalkNodes(s.RootNode(), func(n psi.NodeRef) bool {
		if !overlaps(rng(n), r) {
			return false
		}
		switch n.NumericKind() {
		case psi.KindPROPERTY, psi.KindFUN, psi.KindVALUE_PARAMETER:
			name := n.NameIdentifier()
			if !name.Valid() || n.TypeReference().Valid() {
				return true
			}
			target := name
			sym := i.symbols[name]
			switch n.NumericKind() {
			case psi.KindFUN:
				if !h.returns || !n.ChildKind(psi.KindEQ).Valid() {
					return true
				}
				target = n.ChildKind(psi.KindVALUE_PARAMETER_LIST)
			case psi.KindVALUE_PARAMETER:
				if !h.parameterType {
					return true
				}
			case psi.KindPROPERTY:
				if sym.Kind == "local" && !h.variable || sym.Kind != "local" && !h.property {
					return true
				}
			}
			if t := typeName(&sym.Type); t != "" && target.Valid() {
				add(rng(target).End, ": "+t, 1)
			}
		case psi.KindCALL_EXPRESSION:
			c := i.facts[n].Call
			if !h.arguments || c == nil || !c.StableNames || c.Declaration == nil && !h.compiled {
				return true
			}
			arg := 0
			for a := n.ChildKind(psi.KindVALUE_ARGUMENT_LIST).FirstChild(); a.Valid(); a = a.NextSibling() {
				if a.NumericKind() != psi.KindVALUE_ARGUMENT {
					continue
				}
				if arg < len(c.ArgumentMapping) {
					j := c.ArgumentMapping[arg]
					if j >= 0 && j < len(c.ParameterNames) && !a.ChildKind(psi.KindVALUE_ARGUMENT_NAME).Valid() && a.Text() != c.ParameterNames[j] {
						add(rng(a).Start, c.ParameterNames[j]+":", 2)
					}
				}
				arg++
			}
		}
		return true
	})
	slices.SortStableFunc(out, func(a, b object) int {
		x, y := a["position"].(position), b["position"].(position)
		if before(x, y) {
			return -1
		}
		if before(y, x) {
			return 1
		}
		return 0
	})
	return out
}
func actions(s *psi.Snapshot, i *index, p parameters) []object {
	out := []object{}
	allows := func(kind string) bool {
		return len(p.Context.Only) == 0 || slices.ContainsFunc(p.Context.Only, func(k string) bool { return kind == k || strings.HasPrefix(kind, k+".") })
	}
	action := func(title, kind string, edits []edit) object {
		return object{"title": title, "kind": kind, "edit": object{"changes": map[string][]edit{uri(s.Path()): edits}}}
	}
	if allows("source.organizeImports") {
		if edits := organize(s, i); len(edits) > 0 {
			out = append(out, action("Organize imports", "source.organizeImports", edits))
		}
	}
	if allows("quickfix") {
		for _, d := range diagnostics(s, i) {
			r := d["range"].(span)
			if !overlaps(r, p.Range) {
				continue
			}
			off, _ := offset(s, r.Start)
			n := s.NodeAt(off)
			if code, ok := d["code"].(string); ok && d["severity"] == 2 {
				owner := ancestor(n, psi.KindFUN, psi.KindPROPERTY, psi.KindCLASS)
				if owner.Valid() && !strings.Contains(owner.ChildKind(psi.KindMODIFIER_LIST).Text(), "Suppress") {
					at := owner.NameIdentifier()
					if at.Valid() {
						begin := owner.FirstChild()
						for begin.Valid() && (begin.IsTrivia() || begin.NumericKind() == psi.KindKDOC) {
							begin = begin.NextSibling()
						}
						if begin.Valid() {
							where := rng(begin).Start
							out = append(out, action("Suppress "+code+" for "+owner.Name(), "quickfix", []edit{{span{where, where}, "@Suppress(" + strconv.Quote(code) + ") "}}))
						}
					}
				}
			}
			switch d["code"] {
			case "UNNECESSARY_NOT_NULL_ASSERTION", "UNNECESSARY_SAFE_CALL":
				want, repl := "!!", ""
				if d["code"] == "UNNECESSARY_SAFE_CALL" {
					want, repl = "?.", "."
				}
				if n.Text() == want {
					out = append(out, action("Remove unnecessary "+want, "quickfix", []edit{{rng(n), repl}}))
				}
			}
		}
	}
	if allows("quickfix") {
		start, err := offset(s, p.Range.Start)
		if err == nil {
			n := s.NodeAt(start)
			if n.NumericKind() == psi.KindIDENTIFIER && !i.targets[n].Valid() && n.Parent().NumericKind() == psi.KindREFERENCE_EXPRESSION {
				for _, d := range i.decls {
					if d.Name() != n.Name() || d.Parent().NumericKind() != psi.KindFILE || imported(d, n) || !visibleAt(d, n) || !i.moduleVisible(d, n) {
						continue
					}
					pkg := d.ContainingFile().RootNode().ChildKind(psi.KindPACKAGE_DIRECTIVE).Path()
					if pkg != "" {
						fq := pkg + "." + d.Name()
						out = append(out, action("Import "+fq, "quickfix", addImports(s, map[string]bool{fq: true})))
					}
				}
			}
		}
	}
	return append(out, refactors(s, i, p.Range, allows, action)...)
}
func organize(s *psi.Snapshot, idx *index) []edit {
	used := map[string]bool{}
	var docs []string
	psi.WalkNodes(s.RootNode(), func(n psi.NodeRef) bool {
		switch n.NumericKind() {
		case psi.KindIMPORT_LIST:
			return false
		case psi.KindIDENTIFIER:
			used[n.Name()] = true
		case psi.KindKDOC:
			docs = append(docs, n.Text())
			return false
		}
		return true
	})

	list := s.RootNode().ChildKind(psi.KindIMPORT_LIST)
	var lines []string
	var first, last psi.NodeRef
	for c := list.FirstChild(); c.Valid(); c = c.NextSibling() {
		switch c.NumericKind() {
		case psi.KindIMPORT_DIRECTIVE:
			if !first.Valid() {
				first = c
			}
			last = c
			name := short(c.Path())
			if alias := c.Alias(); alias != "" {
				name = alias
			}
			if idx != nil && len(s.Diagnostics()) == 0 && !c.IsAllUnder() && !used[name] && !slices.ContainsFunc(docs, func(doc string) bool { return strings.Contains(doc, name) }) {
				// Only prune an import whose declaration we identified. An operator
				// function may be used in source without its identifier appearing.
				var id psi.NodeRef
				for q := c.FirstChild(); q.Valid(); q = q.NextSibling() {
					if id = factName(q); id.Valid() {
						break
					}
				}
				decl := idx.targets[id].Parent()
				if decl.Valid() && decl.NumericKind() != psi.KindOBJECT_DECLARATION && !decl.ChildKind(psi.KindMODIFIER_LIST).ChildKind(psi.KindOPERATOR_KEYWORD).Valid() {
					continue
				}
			}
			lines = append(lines, c.Text())
		case psi.KindWHITE_SPACE:
		default:
			return nil
		}
	}
	if !first.Valid() {
		return nil
	}
	slices.Sort(lines)
	lines = slices.Compact(lines)
	sep := "\n"
	if strings.Contains(s.Source(), "\r\n") {
		sep = "\r\n"
	}
	text := strings.Join(lines, sep)
	if text == s.Source()[first.Range().Start:last.Range().End] {
		return nil
	}
	return []edit{{span{rng(first).Start, rng(last).End}, text}}
}

// Format edits whitespace between PSI leaves. Contents of literals and
// comments stay byte-identical, including multiline raw strings.
func format(s *psi.Snapshot, p parameters, limited bool) []edit {
	out := []edit{}
	if len(s.Diagnostics()) > 0 {
		return out
	}
	size := p.Options.TabSize
	if size < 1 || size > 16 {
		size = 4
	}
	unit := "\t"
	if p.Options.InsertSpaces {
		unit = strings.Repeat(" ", size)
	}
	depth := 0
	var prev psi.NodeRef
	psi.WalkNodes(s.RootNode(), func(n psi.NodeRef) bool {
		if !n.IsLeaf() && n.NumericKind() != psi.KindKDOC {
			return true
		}
		if n.NumericKind() == psi.KindWHITE_SPACE {
			return true
		}
		if n.NumericKind() == psi.KindRBRACE {
			depth = max(0, depth-1)
		}
		if prev.Valid() {
			a, b := prev.Range().End, n.Range().Start
			old := s.Source()[a:b]
			want := old
			if j := strings.LastIndexByte(old, '\n'); j >= 0 {
				want = old[:j+1] + strings.Repeat(unit, depth)
			} else if ancestor(n, psi.KindSTRING_TEMPLATE).Valid() && ancestor(n, psi.KindSTRING_TEMPLATE) == ancestor(prev, psi.KindSTRING_TEMPLATE) {
				want = old
			} else if prev.NumericKind() == psi.KindCOMMA || prev.NumericKind() == psi.KindCOLON || prev.NumericKind() == psi.KindEQ || n.NumericKind() == psi.KindEQ || n.NumericKind() == psi.KindLBRACE || binaryOp(n) || binaryOp(prev) {
				want = " "
			} else if n.NumericKind() == psi.KindCOMMA || n.NumericKind() == psi.KindCOLON || n.NumericKind() == psi.KindRPAR || prev.NumericKind() == psi.KindLPAR {
				want = ""
			}
			r := span{pos(s, a), pos(s, b)}
			if old != want && (!limited || !before(r.Start, p.Range.Start) && !before(p.Range.End, r.End)) {
				out = append(out, edit{r, want})
			}
		}
		if n.NumericKind() == psi.KindLBRACE {
			depth++
		}
		prev = n
		return false
	})
	return out
}
func binaryOp(n psi.NodeRef) bool {
	return n.Parent().NumericKind() == psi.KindOPERATION_REFERENCE && n.Parent().Parent().NumericKind() == psi.KindBINARY_EXPRESSION
}

func literal(n psi.NodeRef) bool {
	switch n.NumericKind() {
	case psi.KindINTEGER_CONSTANT, psi.KindBOOLEAN_CONSTANT, psi.KindFLOAT_CONSTANT, psi.KindCHARACTER_CONSTANT, psi.KindNULL:
		return true
	case psi.KindSTRING_TEMPLATE:
		for c := n.FirstChild(); c.Valid(); c = c.NextSibling() {
			if c.NumericKind() == psi.KindLONG_STRING_TEMPLATE_ENTRY || c.NumericKind() == psi.KindSHORT_STRING_TEMPLATE_ENTRY {
				return false
			}
		}
		return true
	}
	return false
}

// Local literal transformations preserve evaluation order. Explicit types,
// delegated properties and inferred calls need additional data-flow checks.
func refactors(s *psi.Snapshot, i *index, r span, allows func(string) bool, action func(string, string, []edit) object) []object {
	out := []object{}
	a, err := offset(s, r.Start)
	if err != nil {
		return out
	}
	b, err := offset(s, r.End)
	if err != nil {
		return out
	}
	at := s.NodeAt(a)
	decl := i.targets[at].Parent()
	if allows("refactor.inline.variable") && decl.NumericKind() == psi.KindPROPERTY && decl.Parent().NumericKind() == psi.KindBLOCK && decl.ChildKind(psi.KindVAL_KEYWORD).Valid() && !decl.TypeReference().Valid() && !decl.ChildKind(psi.KindMODIFIER_LIST).Valid() && literal(decl.Body()) {
		target := decl.NameIdentifier()
		edits := []edit{{rng(decl), ""}}
		safe := true
		psi.WalkNodes(decl.Parent(), func(n psi.NodeRef) bool {
			if n.NumericKind() == psi.KindIDENTIFIER && n.Name() == target.Name() && n != target {
				if i.targets[n] != target || ancestor(n, psi.KindSHORT_STRING_TEMPLATE_ENTRY).Valid() {
					safe = false
				} else {
					edits = append(edits, edit{rng(n), decl.Body().Text()})
				}
			}
			return true
		})
		if safe && len(edits) > 1 {
			out = append(out, action("Inline variable "+decl.Name(), "refactor.inline.variable", edits))
		}
	}
	if !allows("refactor.extract.variable") || a == b {
		return out
	}
	expr := at
	for expr.Valid() && !(expr.Range().Start == a && expr.Range().End == b && literal(expr)) {
		expr = expr.Parent()
	}
	if !expr.Valid() {
		return out
	}
	stmt := expr
	for stmt.Valid() && stmt.Parent().NumericKind() != psi.KindBLOCK {
		stmt = stmt.Parent()
	}
	if !stmt.Valid() || stmt == expr {
		return out
	}
	lineStart := strings.LastIndex(s.Source()[:stmt.Range().Start], "\n") + 1
	indent := s.Source()[lineStart:stmt.Range().Start]
	if strings.Trim(indent, " \t") != "" {
		return out
	}
	names := map[string]bool{}
	psi.WalkNodes(s.RootNode(), func(n psi.NodeRef) bool {
		if n.NumericKind() == psi.KindIDENTIFIER {
			names[n.Name()] = true
		}
		return true
	})
	name := "value"
	for n := 1; names[name]; n++ {
		name = fmt.Sprintf("value%d", n)
	}
	nl := "\n"
	if strings.Contains(s.Source(), "\r\n") {
		nl = "\r\n"
	}
	p := rng(stmt).Start
	return append(out, action("Extract variable "+name, "refactor.extract.variable", []edit{{span{p, p}, "val " + name + " = " + expr.Text() + nl + indent}, {r, name}}))
}

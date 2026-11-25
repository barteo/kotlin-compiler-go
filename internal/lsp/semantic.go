package lsp

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"unicode/utf16"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/analysis/ka"
	"github.com/barteo/kotlin-compiler-go/psi"
)

type index struct {
	workspace *workspace
	model     *ka.EditorModel
	snaps     map[psi.SnapshotID]*psi.Snapshot
	facts     map[psi.NodeRef]analysis.Fact
	symbols   map[psi.NodeRef]analysis.Symbol // exact identifier at cursor
	targets   map[psi.NodeRef]psi.NodeRef     // reference token -> declaration token
	decls     []psi.NodeRef
}

func newIndex(model *ka.EditorModel, snaps []*psi.Snapshot) *index {
	i := &index{model: model, snaps: map[psi.SnapshotID]*psi.Snapshot{}, facts: map[psi.NodeRef]analysis.Fact{}, symbols: map[psi.NodeRef]analysis.Symbol{}, targets: map[psi.NodeRef]psi.NodeRef{}}
	for _, s := range snaps {
		i.snaps[s.ID()] = s
		psi.WalkNodes(s.RootNode(), func(n psi.NodeRef) bool {
			if n.Declaration().Valid() && n.NameIdentifier().Valid() {
				i.decls = append(i.decls, n)
			}
			return true
		})
	}
	data := model.Facts
	classes := map[string]analysis.Symbol{}
	for _, sym := range data.Symbols {
		if sym.Kind == "class" {
			classes[sym.QualifiedName] = sym
		}
	}
	for _, f := range data.Facts {
		n := i.node(&f.Element)
		i.facts[n] = f
		name := factName(n)
		if !name.Valid() {
			continue
		}
		var sym analysis.Symbol
		if f.Symbol != 0 {
			sym = data.Symbols[f.Symbol-1]
		} else if f.Type != nil && (n.NumericKind() == psi.KindTYPE_REFERENCE || n.NumericKind() == psi.KindUSER_TYPE) {
			sym = classes[f.Type.Name]
			if sym.ID == 0 && f.Type.Name != "" && !f.Type.Parameter {
				sym = analysis.Symbol{Kind: "class", Name: strings.ReplaceAll(f.Type.Name, ".", "/"), QualifiedName: f.Type.Name, Type: *f.Type}
			}
		}
		target := i.node(sym.Declaration)
		if !target.Valid() && f.Call != nil {
			target = i.node(f.Call.Declaration)
		}
		if sym.Kind != "" {
			i.symbols[name] = sym
		}
		if target.Valid() {
			if target.NumericKind() == psi.KindPRIMARY_CONSTRUCTOR {
				target = target.Parent()
			}
			if t := target.NameIdentifier(); t.Valid() {
				i.targets[name] = t
			}
		}
	}
	for _, n := range i.decls {
		name := n.NameIdentifier()
		i.targets[name] = name
	}
	// Named argument tokens are references to the selected overload's
	// value parameters, including calls that reorder their arguments.
	for n, f := range i.facts {
		if n.NumericKind() != psi.KindCALL_EXPRESSION || f.Call == nil {
			continue
		}
		c := f.Call
		decl := i.node(c.Declaration)
		if !decl.Valid() {
			continue
		}
		params := decl.Parameters()
		arg := 0
		for a := n.ChildKind(psi.KindVALUE_ARGUMENT_LIST).FirstChild(); a.Valid(); a = a.NextSibling() {
			if a.NumericKind() != psi.KindVALUE_ARGUMENT {
				continue
			}
			id := factName(a.ChildKind(psi.KindVALUE_ARGUMENT_NAME).ChildKind(psi.KindREFERENCE_EXPRESSION))
			if id.Valid() && arg < len(c.ArgumentMapping) {
				j := c.ArgumentMapping[arg]
				if j >= 0 && j < len(params) {
					target := params[j].NameIdentifier()
					if target.Valid() {
						i.targets[id] = target
						i.symbols[id] = i.symbols[target]
					}
				}
			}
			arg++
		}
	}
	// A single explicit import can bind a declaration without producing
	// a checker expression fact. An overloaded import names a family and
	// deliberately stays unresolved for single-symbol refactorings.
	byName := map[string][]psi.NodeRef{}
	for _, d := range i.decls {
		if d.Parent().NumericKind() == psi.KindFILE {
			pkg := d.ContainingFile().RootNode().ChildKind(psi.KindPACKAGE_DIRECTIVE).Path()
			name := d.Name()
			if pkg != "" {
				name = pkg + "." + name
			}
			byName[name] = append(byName[name], d.NameIdentifier())
		}
	}
	for _, s := range snaps {
		for c := s.RootNode().ChildKind(psi.KindIMPORT_LIST).FirstChild(); c.Valid(); c = c.NextSibling() {
			ds := byName[c.Path()]
			if c.IsAllUnder() || len(ds) != 1 {
				continue
			}
			for q := c.FirstChild(); q.Valid(); q = q.NextSibling() {
				if n := factName(q); n.Valid() {
					i.targets[n] = ds[0]
					i.symbols[n] = i.symbols[ds[0]]
					break
				}
			}
		}
	}
	return i
}
func (i *index) node(id *analysis.ElementID) psi.NodeRef {
	if id == nil {
		return psi.NodeRef{}
	}
	return i.snaps[id.Snapshot].Node(id.Node)
}
func factName(n psi.NodeRef) psi.NodeRef {
	if n.Declaration().Valid() || n.NumericKind() == psi.KindREFERENCE_EXPRESSION {
		return n.NameIdentifier()
	}
	switch n.NumericKind() {
	case psi.KindUSER_TYPE:
		return n.ChildKind(psi.KindREFERENCE_EXPRESSION).NameIdentifier()
	case psi.KindTYPE_REFERENCE:
		return factName(n.ChildKind(psi.KindUSER_TYPE))
	case psi.KindDOT_QUALIFIED_EXPRESSION, psi.KindSAFE_ACCESS_EXPRESSION:
		var last psi.NodeRef
		for c := n.FirstChild(); c.Valid(); c = c.NextSibling() {
			if !c.IsLeaf() {
				last = c
			}
		}
		return factName(last)
	case psi.KindCALL_EXPRESSION:
		return factName(n.FirstChild())
	}
	return psi.NodeRef{}
}
func under(n, p psi.NodeRef) bool {
	return p.Valid() && p.ContainingFile() == n.ContainingFile() && n.ID() >= p.ID() && n.ID() < p.SubtreeEnd()
}
func ancestor(n psi.NodeRef, kinds ...psi.Kind) psi.NodeRef {
	for ; n.Valid(); n = n.Parent() {
		if slices.Contains(kinds, n.NumericKind()) {
			return n
		}
	}
	return psi.NodeRef{}
}
func short(s string) string { j := strings.LastIndexAny(s, "./$"); return s[j+1:] }
func typeName(t *analysis.Type) string {
	if t == nil {
		return ""
	}
	if t.Projection == "*" {
		return "*"
	}
	s := short(t.Name)
	if len(t.Arguments) > 0 {
		var args []string
		for _, a := range t.Arguments {
			args = append(args, typeName(&a))
		}
		s += "<" + strings.Join(args, ", ") + ">"
	}
	if t.Nullable {
		s += "?"
	} else if t.Platform {
		s += "!"
	}
	if t.Projection != "" {
		s = t.Projection + " " + s
	}
	return s
}
func summary(n psi.NodeRef) string {
	var b strings.Builder
	for c := n.FirstChild(); c.Valid(); c = c.NextSibling() {
		switch c.NumericKind() {
		case psi.KindKDOC:
			continue
		case psi.KindCLASS_BODY, psi.KindBLOCK, psi.KindEQ:
			return strings.TrimSpace(b.String())
		}
		if c.IsTrivia() {
			if b.Len() > 0 {
				b.WriteByte(' ')
			}
			continue
		}
		b.WriteString(c.Text())
	}
	return strings.TrimSpace(b.String())
}
func kdoc(n psi.NodeRef) string {
	d := n.ChildKind(psi.KindKDOC)
	if !d.Valid() {
		d = n.PreviousSibling()
		for d.NumericKind() == psi.KindWHITE_SPACE {
			d = d.PreviousSibling()
		}
		if d.NumericKind() != psi.KindKDOC {
			return ""
		}
	}
	lines := strings.Split(strings.TrimSuffix(strings.TrimPrefix(d.Text(), "/**"), "*/"), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "*"))
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
func (i *index) hover(n psi.NodeRef) any {
	sym := i.symbols[n]
	t := typeName(&sym.Type)
	d := i.targets[n].Parent()
	label := summary(d)
	if label != "" && t != "" && !d.TypeReference().Valid() && (d.NumericKind() == psi.KindFUN || d.NumericKind() == psi.KindPROPERTY) {
		label += ": " + t
	}
	if label == "" {
		if sym.QualifiedName != "" {
			label = sym.QualifiedName
			if t != "" {
				label += ": " + t
			}
		} else if f := i.facts[n.Parent()]; f.Type != nil {
			label = typeName(f.Type)
		}
	}
	if label == "" {
		return nil
	}
	v := "````kotlin\n" + label + "\n````\n"
	if doc := kdoc(d); doc != "" {
		v = doc + "\n" + v
	}
	return object{"range": rng(n), "contents": object{"kind": "markdown", "value": v}}
}
func symbolKind(n psi.NodeRef) int {
	switch n.NumericKind() {
	case psi.KindCLASS:
		if n.ChildKind(psi.KindINTERFACE_KEYWORD).Valid() {
			return 11
		}
		return 5
	case psi.KindOBJECT_DECLARATION:
		return 19
	case psi.KindFUN:
		if n.Parent().NumericKind() == psi.KindCLASS_BODY {
			return 6
		}
		return 12
	case psi.KindPROPERTY:
		return 7
	case psi.KindTYPE_PARAMETER:
		return 26
	case psi.KindENUM_ENTRY:
		return 22
	case psi.KindTYPEALIAS:
		return 5
	}
	return 13
}
func symbol(n psi.NodeRef) object {
	o := object{"name": n.Name(), "kind": symbolKind(n), "range": rng(n), "selectionRange": rng(n.NameIdentifier())}
	if n.NumericKind() == psi.KindFUN {
		var ts []string
		for _, p := range n.Parameters() {
			ts = append(ts, p.TypeReference().Text())
		}
		detail := "(" + strings.Join(ts, ", ") + ")"
		if rt := n.ReturnType(); rt.Valid() {
			detail += ": " + rt.Text()
		}
		o["detail"] = detail
	}
	return o
}
func outline(s *psi.Snapshot) []object {
	out := []object{}
	parents := map[psi.NodeRef]object{}
	psi.WalkNodes(s.RootNode(), func(n psi.NodeRef) bool {
		if !n.Declaration().Valid() || !n.NameIdentifier().Valid() {
			return true
		}
		switch n.NumericKind() {
		case psi.KindVALUE_PARAMETER, psi.KindTYPE_PARAMETER:
			return false
		}
		item := symbol(n)
		item["deprecated"] = false
		for p := n.Parent(); p.Valid(); p = p.Parent() {
			if parent := parents[p]; parent != nil {
				children, _ := parent["children"].([]object)
				parent["children"] = append(children, item)
				parents[n] = item
				return true
			}
		}
		parents[n] = item
		out = append(out, item)
		return true
	})
	return out
}
func folding(s *psi.Snapshot) []object {
	out := []object{}
	psi.WalkNodes(s.RootNode(), func(n psi.NodeRef) bool {
		kind := "region"
		switch n.NumericKind() {
		case psi.KindIMPORT_LIST:
			kind = "imports"
		case psi.KindKDOC, psi.KindBLOCK_COMMENT:
			kind = "comment"
		case psi.KindCLASS_BODY, psi.KindBLOCK, psi.KindFUNCTION_LITERAL:
		default:
			return true
		}
		r := rng(n)
		if r.End.Line > r.Start.Line {
			item := object{"startLine": r.Start.Line, "startCharacter": r.Start.Character, "endLine": r.End.Line, "endCharacter": r.End.Character, "kind": kind}
			if kind == "region" {
				item["collapsedText"] = "{...}"
			}
			out = append(out, item)
		}
		return true
	})
	return out
}
func (i *index) signature(n psi.NodeRef, off int, labelOffsets bool) any {
	list := ancestor(n, psi.KindVALUE_ARGUMENT_LIST)
	if !list.Valid() && off > 0 {
		list = ancestor(n.ContainingFile().NodeAt(off-1), psi.KindVALUE_ARGUMENT_LIST)
	}
	if !list.Valid() {
		return nil
	}
	call := list.Parent()
	if call.NumericKind() != psi.KindCALL_EXPRESSION {
		return nil
	}
	arg := 0
	for child := list.FirstChild(); child.Valid() && child.Range().Start < off; child = child.NextSibling() {
		if child.NumericKind() == psi.KindCOMMA {
			arg++
		}
	}
	entries := []object{}
	seen := map[string]bool{}
	active := arg
	appendSig := func(name string, labels []string, ret string, decl psi.NodeRef, parameter int) {
		label := name + "(" + strings.Join(labels, ", ") + ")"
		if ret != "" {
			label += ": " + ret
		}
		if seen[label] {
			return
		}
		seen[label] = true
		ps := []object{}
		offset := 0
		if labelOffsets {
			for _, r := range name + "(" {
				offset += utf16.RuneLen(r)
			}
		}
		for _, p := range labels {
			var value any = p
			if labelOffsets {
				end := offset
				for _, r := range p {
					end += utf16.RuneLen(r)
				}
				value = [2]int{offset, end}
				offset = end + 2
			}
			ps = append(ps, object{"label": value})
		}
		sig := object{"label": label, "parameters": ps}
		if len(ps) > 0 {
			sig["activeParameter"] = min(max(parameter, 0), len(ps)-1)
		}
		if doc := decl.ChildKind(psi.KindKDOC); doc.Valid() {
			sig["documentation"] = object{"kind": "markdown", "value": "````kotlin\n" + doc.Text() + "\n````\n"}
		}
		entries = append(entries, sig)
	}
	if c := i.facts[call].Call; c != nil {
		labels := []string{}
		for j, t := range c.Parameters {
			label := typeName(&t)
			if j < len(c.ParameterNames) {
				label = c.ParameterNames[j] + ": " + label
			}
			labels = append(labels, label)
		}
		if arg < len(c.ArgumentMapping) {
			active = c.ArgumentMapping[arg]
		}
		appendSig(short(c.Name), labels, typeName(c.ReturnType), i.node(c.Declaration), active)
	}
	// Before the closing parenthesis, the checker can legitimately have no
	// selected call. Offer visible overloads with their declared signatures.
	callee := factName(call)
	qual := call.Parent()
	qualified := qual.NumericKind() == psi.KindDOT_QUALIFIED_EXPRESSION || qual.NumericKind() == psi.KindSAFE_ACCESS_EXPRESSION
	var receiver *analysis.Type
	if qualified {
		receiver = i.facts[qual.FirstChild()].Type
	}
	for _, d := range i.decls {
		if d.NumericKind() != psi.KindFUN || d.Name() != callee.Name() || !visibleAt(d, callee) || !i.moduleVisible(d, callee) {
			continue
		}
		owner := ancestor(d.Parent(), psi.KindCLASS, psi.KindOBJECT_DECLARATION)
		if qualified {
			if receiver == nil {
				continue
			}
			recv := i.symbols[d.NameIdentifier()].Owner
			if rt := d.ReceiverType(); rt.Valid() {
				if !imported(d, callee) {
					continue
				}
				if t := i.facts[rt].Type; t != nil {
					recv = t.Name
				}
			}
			if !sameType(recv, receiver.Name) && !slices.Contains(receiver.Supertypes, strings.ReplaceAll(recv, "/", ".")) {
				continue
			}
		} else if owner.Valid() && !under(callee, owner) || d.ReceiverType().Valid() || !imported(d, callee) {
			continue
		}
		params := d.Parameters()
		if len(params) <= arg && !(arg == 0 && len(params) == 0) && !(len(params) > 0 && params[len(params)-1].ChildKind(psi.KindMODIFIER_LIST).ChildKind(psi.KindVARARG_KEYWORD).Valid()) {
			continue
		}
		labels := []string{}
		for _, p := range params {
			labels = append(labels, p.Name()+": "+p.TypeReference().Text())
		}
		sym := i.symbols[d.NameIdentifier()]
		ret := typeName(&sym.Type)
		if ret == "" {
			ret = d.ReturnType().Text()
		}
		appendSig(d.Name(), labels, ret, d, arg)
	}
	if len(entries) == 0 {
		return nil
	}
	out := object{"signatures": entries, "activeSignature": 0}
	if a, ok := entries[0]["activeParameter"]; ok {
		out["activeParameter"] = a
	}
	return out
}
func sameType(a, b string) bool {
	return strings.ReplaceAll(strings.ReplaceAll(a, "/", "."), "$", ".") == b
}
func visibleAt(d, at psi.NodeRef) bool {
	if sc := ancestor(d.Parent(), psi.KindBLOCK, psi.KindFUNCTION_LITERAL); sc.Valid() && (!under(at, sc) || d.Range().Start > at.Range().Start) {
		return false
	}
	if d.NumericKind() == psi.KindVALUE_PARAMETER {
		owner := d.Parent().Parent()
		if owner.NumericKind() == psi.KindPRIMARY_CONSTRUCTOR {
			owner = owner.Parent()
		}
		if !under(at, owner) {
			return false
		}
	}
	if d.ChildKind(psi.KindMODIFIER_LIST).ChildKind(psi.KindPRIVATE_KEYWORD).Valid() {
		if cl := ancestor(d.Parent(), psi.KindCLASS, psi.KindOBJECT_DECLARATION); cl.Valid() {
			return under(at, cl)
		}
		return d.ContainingFile() == at.ContainingFile()
	}
	return true
}
func imported(d, at psi.NodeRef) bool {
	pkg := d.ContainingFile().RootNode().ChildKind(psi.KindPACKAGE_DIRECTIVE).Path()
	root := at.ContainingFile().RootNode()
	if pkg == root.ChildKind(psi.KindPACKAGE_DIRECTIVE).Path() {
		return true
	}
	for c := root.ChildKind(psi.KindIMPORT_LIST).FirstChild(); c.Valid(); c = c.NextSibling() {
		if c.Alias() == "" && (c.IsAllUnder() && c.Path() == pkg || c.Path() == pkg+"."+d.Name()) {
			return true
		}
	}
	return false
}
func (i *index) completion(ctx context.Context, s *psi.Snapshot, off int) []object {
	at := s.NodeAt(max(off-1, 0))
	prefix := ""
	end := off
	if at.NumericKind() == psi.KindIDENTIFIER {
		prefix = s.Source()[at.Range().Start:off]
		end = at.Range().Start
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

			if receiver.Name == "" && len(s.Diagnostics()) > 0 {
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
	out := []object{}
	seen := map[string]bool{}
	add := func(name, detail string, kind, rank int) object {
		key := name + "\n" + detail
		if name == "" || seen[key] || !strings.HasPrefix(name, prefix) {
			return nil
		}
		seen[key] = true
		item := object{"label": name, "detail": detail, "kind": kind, "sortText": fmt.Sprintf("%02d-%s", rank, name)}
		out = append(out, item)
		return item
	}
	root := s.RootNode()
	aliases := map[string]string{}
	localNames := map[string]string{}
	for imp := root.ChildKind(psi.KindIMPORT_LIST).FirstChild(); imp.Valid(); imp = imp.NextSibling() {
		if !imp.Import().Valid() || imp.IsAllUnder() {
			continue
		}
		name := short(imp.Path())
		if alias := imp.Alias(); alias != "" {
			aliases[imp.Path()] = alias
			name = alias
		}
		localNames[name] = imp.Path()
	}
	for _, d := range i.decls {
		if !visibleAt(d, at) || !i.moduleVisible(d, at) {
			continue
		}
		sym := i.symbols[d.NameIdentifier()]
		name := d.Name()
		autoImport := ""
		if d.Parent().NumericKind() == psi.KindFILE {
			pkg := d.ContainingFile().RootNode().ChildKind(psi.KindPACKAGE_DIRECTIVE).Path()
			fq := pkg + "." + name
			if alias := aliases[fq]; alias != "" {
				name = alias
			} else if !imported(d, at) {
				if prefix == "" || pkg == "" || localNames[name] != "" || !strings.HasPrefix(name, prefix) {
					continue
				}
				autoImport = fq
			}
		}
		outer := ancestor(d.Parent(), psi.KindCLASS, psi.KindOBJECT_DECLARATION)
		if member {
			if receiver.Name == "" {
				continue
			}
			if rt := d.ReceiverType(); rt.Valid() {
				t := i.facts[rt].Type
				name := rt.Text()
				if t != nil {
					name = t.Name
				}
				if name != short(receiver.Name) && !sameType(name, receiver.Name) && !slices.Contains(receiver.Supertypes, name) {
					continue
				}
			} else if !sameType(sym.Owner, receiver.Name) && !slices.Contains(receiver.Supertypes, strings.ReplaceAll(sym.Owner, "/", ".")) {
				continue
			}
		} else if outer.Valid() && !under(at, outer) || d.ReceiverType().Valid() || d.Parent().NumericKind() != psi.KindFILE && !imported(d, at) {
			continue
		}
		rank := 2
		if d.ContainingFile() == s {
			rank = 1
		}
		if sym.Kind == "parameter" || sym.Kind == "local" {
			rank = 0
		}
		k := 6
		switch symbolKind(d) {
		case 5, 11:
			k = 7
		case 6, 12:
			k = 3
		case 7:
			k = 10
		}
		detail := summary(d)
		if autoImport != "" {
			if item := add(name, detail+" ("+autoImport+")", k, 4); item != nil {
				item["additionalTextEdits"] = addImports(s, map[string]bool{autoImport: true})
			}
		} else {
			add(name, detail, k, rank)
		}
	}
	if member && receiver.Name != "" {
		for _, name := range append([]string{receiver.Name}, receiver.Supertypes...) {
			if cl := i.model.Class(strings.ReplaceAll(name, ".", "/")); cl != nil {
				for _, m := range cl.Members {
					if m.Visibility == ka.Public && (m.Kind == ka.KindFunction || m.Kind == ka.KindProperty) {
						k := 2
						if m.Kind == ka.KindProperty {
							k = 10
						}
						add(m.Name, librarySignature(m), k, 2)
					}
				}
			}
		}
	}
	// Enumerate package callables from Kotlin metadata, including imported
	// and default-imported extensions applicable to the resolved receiver.
	pkgs := map[string]bool{"kotlin": true, "kotlin.collections": true, "kotlin.io": true, "kotlin.ranges": true, "kotlin.sequences": true, "kotlin.text": true}
	pkgs[root.ChildKind(psi.KindPACKAGE_DIRECTIVE).Path()] = true
	explicit := map[string]string{}
	for c := root.ChildKind(psi.KindIMPORT_LIST).FirstChild(); c.Valid(); c = c.NextSibling() {
		if !c.Import().Valid() {
			continue
		}
		if c.IsAllUnder() {
			pkgs[c.Path()] = true
		} else if pos := strings.LastIndexByte(c.Path(), '.'); pos >= 0 {
			pkg := c.Path()[:pos]
			if _, ok := pkgs[pkg]; !ok {
				pkgs[pkg] = false
			}
			alias := c.Alias()
			if alias == "" {
				alias = short(c.Path())
			}
			explicit[c.Path()] = alias
		}
	}
	for pkg, star := range pkgs {
		if ctx.Err() != nil {
			break
		}
		lib := i.model.Package(pkg)
		if lib == nil {
			continue
		}
		for _, d := range lib.Decls {
			if d.Visibility != ka.Public {
				continue
			}
			alias := explicit[pkg+"."+d.Name]
			if !star && alias == "" {
				continue
			}
			if member != (d.Receiver != nil) {
				continue
			}
			if d.Receiver != nil {
				recv := strings.ReplaceAll(d.Receiver.ClassID, "/", ".")
				if receiver.Name == "" || recv != receiver.Name && !slices.Contains(receiver.Supertypes, recv) {
					continue
				}
			}
			name := d.Name
			if alias != "" {
				name = alias
			}
			if !strings.HasPrefix(name, prefix) {
				continue
			}
			kind := 3
			if d.Kind == ka.KindProperty {
				kind = 10
			}
			add(name, librarySignature(d), kind, 3)
		}
		if !member {
			for _, cl := range lib.Classes {
				name := short(cl)
				alias := explicit[pkg+"."+name]
				if !star && alias == "" {
					continue
				}
				if alias != "" {
					name = alias
				}
				add(name, strings.ReplaceAll(cl, "/", "."), 7, 3)
			}
		}
	}
	slices.SortStableFunc(out, func(a, b object) int {
		return cmp.Or(strings.Compare(a["sortText"].(string), b["sortText"].(string)), strings.Compare(a["detail"].(string), b["detail"].(string)))
	})
	return out
}

func (i *index) moduleVisible(d, at psi.NodeRef) bool {
	src := i.workspace.module(at.ContainingFile().Path())
	dst := i.workspace.module(d.ContainingFile().Path())
	if src == dst {
		return true
	}
	visible := i.workspace.visible[src]
	if visible == nil {
		visible = i.workspace.p.visibleModules(src)
		i.workspace.visible[src] = visible
	}
	if !visible[dst] {
		return false
	}
	for n := d; n.Valid(); n = n.Parent() {
		if n.ChildKind(psi.KindMODIFIER_LIST).ChildKind(psi.KindINTERNAL_KEYWORD).Valid() {
			return slices.Contains(src.FriendModules, dst.Name)
		}
	}
	return true
}

func libraryType(t *ka.Type) string {
	if t == nil {
		return ""
	}
	name := short(t.ClassID)
	if name == "" {
		name = t.Name
	}
	if len(t.Args) > 0 {
		var args []string
		for _, a := range t.Args {
			if a.Type == nil {
				args = append(args, "*")
			} else {
				args = append(args, libraryType(a.Type))
			}
		}
		name += "<" + strings.Join(args, ", ") + ">"
	}
	if t.Nullable {
		name += "?"
	}
	return name
}
func librarySignature(m *ka.Decl) string {
	sig := m.Name
	if m.Kind == ka.KindFunction {
		var params []string
		for _, p := range m.Params {
			params = append(params, p.Name+": "+libraryType(p.Type))
		}
		sig += "(" + strings.Join(params, ", ") + ")"
	}
	if m.Type != nil {
		sig += ": " + libraryType(m.Type)
	}
	return sig
}

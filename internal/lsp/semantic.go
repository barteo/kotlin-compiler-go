package lsp

import (
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/analysis/ka"
	"github.com/barteo/kotlin-compiler-go/psi"
)

type index struct {
	importEdits map[*psi.Snapshot][]edit
	workspace   *workspace
	model       *ka.EditorModel
	snaps       map[psi.SnapshotID]*psi.Snapshot
	facts       map[psi.NodeRef]analysis.Fact
	symbols     map[psi.NodeRef]analysis.Symbol // exact identifier at cursor
	targets     map[psi.NodeRef]psi.NodeRef     // reference token -> declaration token
	byReceiver  map[string][]psi.NodeRef
	decls       []psi.NodeRef
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
	i.byReceiver = map[string][]psi.NodeRef{}
	for _, n := range i.decls {
		name := n.NameIdentifier()
		var receiver string
		if rt := n.ReceiverType(); rt.Valid() {
			if t := i.facts[rt].Type; t != nil {
				receiver = t.Name
			}
		} else if constructorProperty(n) {
			receiver = i.symbols[n.Parent().Parent().Parent().NameIdentifier()].QualifiedName
		} else if n.Parent().NumericKind() == psi.KindCLASS_BODY {
			receiver = dottedName(i.symbols[name].Owner)
		}
		if receiver != "" {
			i.byReceiver[receiver] = append(i.byReceiver[receiver], n)
		}
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

// sameType compares the classifier and type arguments, independent of the
// source spelling or the symbol IDs used in two different editor models.
func sameType(a, b *analysis.Type) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Name != b.Name || a.Nullable != b.Nullable || a.Projection != b.Projection ||
		a.Parameter != b.Parameter || a.Platform != b.Platform || a.Suspend != b.Suspend || len(a.Arguments) != len(b.Arguments) {
		return false
	}
	for j := range a.Arguments {
		if !sameType(&a.Arguments[j], &b.Arguments[j]) {
			return false
		}
	}
	return a.Name != "" || a.Projection == "*"
}

func typeName(t *analysis.Type) string {
	var out strings.Builder
	typeText(t, func(text, _ string) { out.WriteString(text) })
	return out.String()
}

// Emit the displayed type and keep each classifier's qualified identity for navigation.
func typeText(t *analysis.Type, emit func(text, classifier string)) {
	if t == nil {
		return
	}
	if t.Projection == "*" {
		emit("*", "")
		return
	}
	if t.Projection != "" {
		emit(t.Projection+" ", "")
	}
	classifier := t.Name
	if t.Parameter {
		classifier = ""
	}
	emit(short(t.Name), classifier)
	if len(t.Arguments) > 0 {
		emit("<", "")
		for j := range t.Arguments {
			if j > 0 {
				emit(", ", "")
			}
			typeText(&t.Arguments[j], emit)
		}
		emit(">", "")
	}
	if t.Nullable {
		emit("?", "")
	} else if t.Platform {
		emit("!", "")
	}
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
	var detail string
	switch n.NumericKind() {
	case psi.KindFUN, psi.KindPROPERTY, psi.KindVALUE_PARAMETER:
		if rt := n.ReceiverType(); rt.Valid() {
			detail = rt.Text() + "."
		}
		if n.NumericKind() == psi.KindFUN {
			detail += outlineParameters(n)
		}
		if rt := n.ReturnType(); rt.Valid() {
			detail += ": " + rt.Text()
		}
	}
	if detail != "" {
		o["detail"] = detail
	}
	return o
}

func outlineParameters(n psi.NodeRef) string {
	var types []string
	for _, p := range n.Parameters() {
		types = append(types, p.TypeReference().Text())
	}
	return "(" + strings.Join(types, ", ") + ")"
}

func outline(s *psi.Snapshot) []object {
	out := []object{}
	parents := map[psi.NodeRef]object{}
	add := func(n psi.NodeRef, item object) {
		item["deprecated"] = false
		for p := n.Parent(); p.Valid(); p = p.Parent() {
			if parent := parents[p]; parent != nil {
				children, _ := parent["children"].([]object)
				parent["children"] = append(children, item)
				return
			}
		}
		out = append(out, item)
	}
	psi.WalkNodes(s.RootNode(), func(n psi.NodeRef) bool {
		switch n.NumericKind() {
		case psi.KindPRIMARY_CONSTRUCTOR:
			for _, p := range n.Parameters() {
				if constructorProperty(p) {
					add(p, symbol(p))
				}
			}
			add(n, object{"name": n.Parent().Name(), "kind": 9, "detail": outlineParameters(n), "range": rng(n), "selectionRange": rng(n)})
			return false
		case psi.KindPROPERTY_ACCESSOR:
			name := "get"
			if n.ChildKind(psi.KindSET_KEYWORD).Valid() {
				name = "set"
			}
			add(n, object{"name": name, "kind": 6, "range": rng(n), "selectionRange": rng(n)})
			return false
		case psi.KindTYPE_PARAMETER, psi.KindVALUE_PARAMETER:
			return false
		}
		if n.Declaration().Valid() && n.NameIdentifier().Valid() {
			item := symbol(n)
			add(n, item)
			parents[n] = item
		}
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
func visibleAt(d, at psi.NodeRef) bool {
	if sc := ancestor(d.Parent(), psi.KindBLOCK, psi.KindFUNCTION_LITERAL); sc.Valid() && (!under(at, sc) || d.Range().Start > at.Range().Start) {
		return false
	}
	if d.NumericKind() == psi.KindVALUE_PARAMETER && !constructorProperty(d) {
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

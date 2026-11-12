package ide

import (
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// An importDirective is an import of a file.
type importDirective struct {
	node  psi.NodeRef
	path  string // the dotted path, without .*
	star  bool
	alias string
}

// local returns the name the import brings into scope; "" for a star
// import.
func (i importDirective) local() string {
	if i.star {
		return ""
	}
	if i.alias != "" {
		return i.alias
	}
	return i.path[strings.LastIndexByte(i.path, '.')+1:]
}

// text is the directive as organize imports writes it.
func (i importDirective) text() string {
	s := "import " + quotePath(i.path)
	if i.star {
		s += ".*"
	}
	if i.alias != "" {
		a, _ := psi.QuoteName(i.alias)
		s += " as " + a
	}
	return s
}

func quotePath(path string) string {
	parts := strings.Split(path, ".")
	for i, p := range parts {
		parts[i], _ = psi.QuoteName(p)
	}
	return strings.Join(parts, ".")
}

// imports returns the import directives of snap.
func imports(snap *psi.Snapshot) []importDirective {
	var out []importDirective
	for c := snap.RootNode().ChildKind(psi.KindIMPORT_LIST).FirstChild(); c.Valid(); c = c.NextSibling() {
		if c.NumericKind() != psi.KindIMPORT_DIRECTIVE {
			continue
		}
		names, _ := importPath(c)
		if len(names) == 0 {
			continue
		}
		d := importDirective{node: c, path: strings.Join(names, ".")}
		if a := c.ChildKind(psi.KindIMPORT_ALIAS); a.Valid() {
			if id := a.NameIdentifier(); id.Valid() {
				d.alias = psi.Unquote(id.Text())
			} else {
				psi.WalkNodes(a, func(n psi.NodeRef) bool {
					if n.NumericKind() == psi.KindIDENTIFIER {
						d.alias = psi.Unquote(n.Text())
					}
					return true
				})
			}
		}
		d.star = strings.HasSuffix(strings.TrimSpace(c.Text()), "*")
		out = append(out, d)
	}
	return out
}

// importGroup is the group of IntelliJ's default Kotlin import layout an
// import sorts in: other imports, java.*, javax.*, kotlin.*, then alias
// imports.
func importGroup(path string, alias bool) int {
	switch {
	case alias:
		return 4
	case path == "java" || strings.HasPrefix(path, "java."):
		return 1
	case path == "javax" || strings.HasPrefix(path, "javax."):
		return 2
	case path == "kotlin" || strings.HasPrefix(path, "kotlin."):
		return 3
	}
	return 0
}

// importLess orders imports as IntelliJ's layout does: by group, then by
// path, a star import of a path before the imports under it.
func importLess(a, b importDirective) bool {
	ga, gb := importGroup(a.path, a.alias != ""), importGroup(b.path, b.alias != "")
	if ga != gb {
		return ga < gb
	}
	if a.path != b.path {
		return a.path < b.path
	}
	if a.star != b.star {
		return a.star
	}
	return a.alias < b.alias
}

// ImportEdit returns the edit that imports fq into snap, at its sorted
// place among the file's imports when they are sorted, else after them.
func ImportEdit(snap *psi.Snapshot, fq string) TextEdit {
	imps := imports(snap)
	add := importDirective{path: fq}
	if len(imps) > 0 {
		sorted := slices.IsSortedFunc(imps, func(a, b importDirective) int {
			if importLess(a, b) {
				return -1
			}
			if importLess(b, a) {
				return 1
			}
			return 0
		})
		if sorted {
			for _, i := range imps {
				if importLess(add, i) {
					at := i.node.Range().Start
					return TextEdit{Range: psi.Range{Start: at, End: at}, Text: add.text() + "\n"}
				}
			}
		}
		at := lastImportEnd(snap)
		return TextEdit{Range: psi.Range{Start: at, End: at}, Text: "\n" + add.text()}
	}
	for c := snap.RootNode().FirstChild(); c.Valid(); c = c.NextSibling() {
		if c.NumericKind() == psi.KindPACKAGE_DIRECTIVE && strings.TrimSpace(c.Text()) != "" {
			at := c.Range().End
			return TextEdit{Range: psi.Range{Start: at, End: at}, Text: "\n\n" + add.text()}
		}
	}
	return TextEdit{Text: add.text() + "\n\n"}
}

func lastImportEnd(snap *psi.Snapshot) int {
	end := -1
	for c := snap.RootNode().ChildKind(psi.KindIMPORT_LIST).FirstChild(); c.Valid(); c = c.NextSibling() {
		if c.NumericKind() == psi.KindIMPORT_DIRECTIVE {
			end = c.Range().End
		}
	}
	return end
}

// operatorNames are the names a call by convention may use without
// writing them, which an import may bring into scope: x + y, a[i],
// by lazy, for loops, destructuring and so on.
var operatorNames = map[string]bool{
	"plus": true, "minus": true, "times": true, "div": true, "rem": true, "mod": true,
	"rangeTo": true, "rangeUntil": true, "contains": true, "get": true, "set": true,
	"invoke": true, "getValue": true, "setValue": true, "provideDelegate": true,
	"iterator": true, "next": true, "hasNext": true, "compareTo": true, "equals": true,
	"inc": true, "dec": true, "unaryPlus": true, "unaryMinus": true, "not": true,
	"plusAssign": true, "minusAssign": true, "timesAssign": true, "divAssign": true,
	"remAssign": true, "modAssign": true,
}

// UnusedImports returns the imports of the file at path that nothing in
// it uses, as far as the view tells: none of the file's references,
// types or calls resolves through them, and no KDoc links to them. It
// errs on the side of used: a file with diagnostics, a name with no
// facts, or a name an operator convention may call reports nothing.
func UnusedImports(v *View, path string) []psi.Range {
	snap := v.byPath[path]
	if snap == nil {
		return nil
	}
	for _, d := range v.res.Diagnostics() {
		if d.Path == path {
			return nil
		}
	}
	imps := imports(snap)
	if len(imps) == 0 {
		return nil
	}
	u := usesOf(v, snap)
	var out []psi.Range
	seen := map[string]bool{}
	for _, i := range imps {
		key := i.text()
		dup := seen[key]
		seen[key] = true
		if !dup && u.uses(i) {
			continue
		}
		out = append(out, i.node.Range())
	}
	return out
}

// fileUses are the names a file's references resolve to.
type fileUses struct {
	// byName are, for each name written, what its occurrences resolve
	// to: fully qualified names, "" for a resolved occurrence that names
	// nothing an import provides; nil for an occurrence with no fact.
	byName     map[string][][]string
	unknown    map[string]bool // names with an occurrence without facts
	all        map[string]bool // every fully qualified name resolved to
	anyUnknown bool
	kdoc       []string
	aliases    map[string]bool // the source typealiases' qualified names
}

func (u *fileUses) uses(i importDirective) bool {
	if i.star {
		if u.anyUnknown || u.aliases[i.path] {
			return true // a typealias's members resolve to its class's
		}
		for fq := range u.all {
			if parent(fq) == i.path {
				return true
			}
		}
		return false
	}
	name := i.local()
	if operatorNames[name] || strings.HasPrefix(name, "component") || u.unknown[name] || kdocMentions(u.kdoc, name) {
		return true
	}
	for _, names := range u.byName[name] {
		through := true // resolved through an alias: no name ends in name
		for _, fq := range names {
			if fq == i.path || strings.HasPrefix(fq, i.path+".") {
				return true
			}
			if fq == "" || fq == name || strings.HasSuffix(fq, "."+name) {
				through = false
			}
		}
		if through {
			return true
		}
	}
	return false
}

func parent(fq string) string {
	if i := strings.LastIndexByte(fq, '.'); i >= 0 {
		return fq[:i]
	}
	return ""
}

// kdocMentions reports whether a KDoc links to name: [name] or
// [name.member].
func kdocMentions(kdoc []string, name string) bool {
	for _, t := range kdoc {
		if strings.Contains(t, "["+name+"]") || strings.Contains(t, "["+name+".") {
			return true
		}
	}
	return false
}

// usesOf collects the uses of snap's names outside its package directive
// and imports.
func usesOf(v *View, snap *psi.Snapshot) *fileUses {
	u := &fileUses{byName: map[string][][]string{}, unknown: map[string]bool{}, all: map[string]bool{}, aliases: map[string]bool{}}
	v.classIndex()
	for fq := range v.aliases {
		u.aliases[fq] = true
	}
	psi.WalkNodes(snap.RootNode(), func(n psi.NodeRef) bool {
		switch n.NumericKind() {
		case psi.KindPACKAGE_DIRECTIVE, psi.KindIMPORT_LIST:
			return false
		case psi.KindKDOC:
			u.kdoc = append(u.kdoc, n.Text())
			return false
		case psi.KindIDENTIFIER:
			p := n.Parent()
			if k := p.NumericKind(); k != psi.KindREFERENCE_EXPRESSION && k != psi.KindOPERATION_REFERENCE && p.NameIdentifier() == n {
				return false // a declaration's name
			}
			switch p.NumericKind() {
			case psi.KindLABEL, psi.KindLABEL_QUALIFIER, psi.KindVALUE_ARGUMENT_NAME:
				return false
			}
			name := psi.Unquote(n.Text())
			names, ok := v.resolved(n)
			if !ok {
				u.unknown[name] = true
				u.anyUnknown = true
				return false
			}
			u.byName[name] = append(u.byName[name], names)
			for _, fq := range names {
				if fq != "" {
					u.all[fq] = true
				}
			}
			return false
		}
		return true
	})
	// Operators, delegates, loops and destructuring call by convention.
	for f := range v.res.FileFacts(snap.ID()) {
		for _, fq := range factNames(v, f, true) {
			u.all[fq] = true
		}
	}
	return u
}

// resolved returns what the identifier id resolves to, from the facts
// of the reference it is in and of the call or type around that: fully
// qualified names, or "" for a resolved name such as a local's. It
// reports false if none of them has a fact that resolves anything.
func (v *View) resolved(id psi.NodeRef) ([]string, bool) {
	var names []string
	found := false
	n := id.Parent()
	for range 2 {
		if !n.Valid() {
			break
		}
		if f, ok := v.fact(n); ok && (f.Symbol != 0 || f.Call != nil || f.Type != nil || f.ImportableName != "" || f.Package != "" || len(f.Accessors) > 0) {
			found = true
			names = append(names, factNames(v, f, false)...)
		}
		switch n.Parent().NumericKind() {
		case psi.KindCALL_EXPRESSION, psi.KindUSER_TYPE, psi.KindCALLABLE_REFERENCE_EXPRESSION, psi.KindBINARY_EXPRESSION:
			n = n.Parent()
			continue
		}
		break
	}
	if found && len(names) == 0 {
		names = []string{""}
	}
	return names, found
}

// factNames returns the fully qualified names a fact resolves to: its
// call's callee and other targets, its delegated and accessor calls, its
// symbol's, its classifier's and, on a type, the type's class. With
// conventions, only the calls a construct makes without naming them.
func factNames(v *View, f analysis.Fact, conventions bool) []string {
	var out []string
	call := func(c analysis.Call) {
		out = append(out, callName(c.Name))
		for _, t := range c.Targets {
			out = append(out, callName(t.Name))
		}
	}
	for _, c := range f.DelegatedCalls {
		call(c)
	}
	if conventions {
		if f.Call != nil && f.Call.Operator {
			call(*f.Call)
		}
		return out
	}
	if f.Call != nil {
		call(*f.Call)
	}
	for _, c := range f.Accessors {
		call(c)
	}
	if f.ImportableName != "" {
		out = append(out, f.ImportableName)
	}
	if s, ok := v.symbol(f.Symbol); ok && s.QualifiedName != "" {
		out = append(out, s.QualifiedName)
	}
	if f.Type != nil && f.Type.Name != "" && (f.Classifier || f.Role == "type") {
		out = append(out, f.Type.Name)
	}
	return out
}

// callName is the fully qualified name of a callee as an import names
// it: a constructor's class, an accessor's property.
func callName(name string) string {
	if p, ok := strings.CutSuffix(name, ".<init>"); ok {
		return p
	}
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		last := name[i+1:]
		if p, ok := strings.CutPrefix(last, "<get-"); ok {
			return name[:i+1] + strings.TrimSuffix(p, ">")
		}
		if p, ok := strings.CutPrefix(last, "<set-"); ok {
			return name[:i+1] + strings.TrimSuffix(p, ">")
		}
	}
	return name
}

// OrganizeImports returns the edit that removes the unused and
// duplicate imports of the file at path and sorts the others in
// IntelliJ's default layout, or false if there is nothing to change or
// a comment among the imports would be lost.
func OrganizeImports(v *View, path string) (TextEdit, bool) {
	snap := v.byPath[path]
	if snap == nil {
		return TextEdit{}, false
	}
	imps := imports(snap)
	if len(imps) == 0 {
		return TextEdit{}, false
	}
	start, end := imps[0].node.Range().Start, imps[len(imps)-1].node.Range().End
	for c := snap.RootNode().ChildKind(psi.KindIMPORT_LIST).FirstChild(); c.Valid(); c = c.NextSibling() {
		r := c.Range()
		if r.Start < start || r.End > end {
			continue
		}
		if c.NumericKind() != psi.KindIMPORT_DIRECTIVE && strings.TrimSpace(c.Text()) != "" || hasComment(c) {
			return TextEdit{}, false // a comment
		}
	}
	unused := map[int]bool{}
	for _, r := range UnusedImports(v, path) {
		unused[r.Start] = true
	}
	var keep []importDirective
	seen := map[string]bool{}
	for _, i := range imps {
		if unused[i.node.Range().Start] || seen[i.text()] {
			continue
		}
		seen[i.text()] = true
		keep = append(keep, i)
	}
	slices.SortStableFunc(keep, func(a, b importDirective) int {
		if importLess(a, b) {
			return -1
		}
		if importLess(b, a) {
			return 1
		}
		return 0
	})
	src := snap.Source()
	var e TextEdit
	if len(keep) == 0 {
		// Remove the list and the blank lines after it.
		to := end
		for to < len(src) && (src[to] == '\n' || src[to] == ' ' || src[to] == '\t' || src[to] == '\r') {
			to++
		}
		e = TextEdit{Range: psi.Range{Start: start, End: to}}
	} else {
		lines := make([]string, len(keep))
		for i, k := range keep {
			lines[i] = k.text()
		}
		e = TextEdit{Range: psi.Range{Start: start, End: end}, Text: strings.Join(lines, "\n")}
	}
	if src[e.Range.Start:e.Range.End] == e.Text {
		return TextEdit{}, false
	}
	return e, true
}

// hasComment reports whether n holds a comment.
func hasComment(n psi.NodeRef) bool {
	found := false
	psi.WalkNodes(n, func(m psi.NodeRef) bool {
		switch m.NumericKind() {
		case psi.KindEOL_COMMENT, psi.KindBLOCK_COMMENT, psi.KindKDOC:
			found = true
		}
		return !found
	})
	return found
}

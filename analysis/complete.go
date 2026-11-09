package analysis

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"unicode"

	"github.com/barteo/kotlin-compiler-go/internal/check"
	"github.com/barteo/kotlin-compiler-go/internal/lower"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// ProbeName is the name an editor puts in place of the name being typed
// at its caret, or at the caret, to complete it: Session.Complete reports
// what the code may name there.
const ProbeName = check.ProbeName

// Completion is what the code may name at a probe.
type Completion struct {
	// Reached is false if checking didn't reach the probe: lowering
	// dropped the code around it, or it isn't where a name goes. The
	// rest is then empty.
	Reached bool
	// Expected is the type expected of the probe's value, if known.
	Expected *Type
	// TypePosition is set when the probe is a type, Callable on a
	// callable reference: x::probe.
	TypePosition bool
	Callable     bool
	// Receiver is the type of the explicit receiver, x in x.probe or
	// x::probe, and Safe is set on x?.probe. Static is the fully
	// qualified name of the class whose name is the receiver, and Package
	// the package's.
	Receiver *Type
	Safe     bool
	Static   string
	Package  string
	// Receivers are the types of the implicit receivers, innermost first.
	Receivers []Type
	// Items are what the probe may name, innermost scope first.
	Items []CompletionItem
	// Calls are the calls whose argument the probe is, innermost first.
	Calls []CompletionCall
	// Importables are the class path's declarations the probe may name
	// after an import (Importables).
	Importables []Importable

	snaps []*psi.Snapshot
}

// CompletionItem is one thing a probe may name.
type CompletionItem struct {
	Name string
	// Kind is "local", "parameter", "property", "function", "class",
	// "interface", "object", "enum", "annotation", "typealias",
	// "typeParameter" or "package".
	Kind string
	// Origin is where it comes from: "local", "member", "extension",
	// "topLevel", "classifier", "typeParameter", "package" or "static".
	Origin string
	// QualifiedName is a class's, a typealias's or a package's, or a
	// member's or top-level function's or property's.
	QualifiedName string
	// Type is a variable's or property's type, a function's return type,
	// or a class's own type, with the receiver's type arguments.
	Type *Type
	// Call is a function's, a property getter's or a constructor's
	// signature, with the receiver's type arguments.
	Call *Call
	// Receiver is the index in Completion.Receivers of the implicit
	// receiver whose member or extension it is, or -1.
	Receiver   int
	Deprecated bool
	// Declaration is its declaration, if it is in the sources.
	Declaration *ElementID
}

// CompletionCall is a call a probe is an argument of.
type CompletionCall struct {
	// Candidates are the functions the call may call, by its name.
	Candidates []Call
	// Argument is the index of the probe's argument, and Name its name,
	// if it is named; Names are the call's named arguments' names.
	Argument int
	Name     string
	Names    []string
}

// Importable is a declaration of the class path that needs an import.
type Importable struct {
	Name, Package string
	Class         bool // a class or typealias; otherwise a function or property
}

// Node returns the psi node of el, a declaration Complete reported; the
// node is in the snapshots Complete was given.
func (c *Completion) Node(el ElementID) psi.NodeRef {
	for _, s := range c.snaps {
		if s.ID() == el.Snapshot {
			return s.Node(el.Node)
		}
	}
	return psi.NodeRef{}
}

// probeRecorder is the recorder of a probe check.
type probeRecorder struct {
	*recorder
	mu   sync.Mutex
	view *check.ScopeView
}

func (r *probeRecorder) Scope(v *check.ScopeView) {
	r.mu.Lock()
	r.view = v
	r.mu.Unlock()
}

// Complete reports what the code may name at the probe in the file at
// path: snapshots are the module's sources, that file with ProbeName in
// place of the name being typed. It checks that file only and doesn't
// change what the session's next Update starts from. Importables are
// listed only when prefix, the part of the name typed before the caret,
// has a letter: those whose name starts with its first letter, in any
// case.
func (s *Session) Complete(ctx context.Context, snapshots []*psi.Snapshot, path, prefix string) (*Completion, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validate(s.cfg, snapshots); err != nil {
		return nil, err
	}
	at := -1
	for i, snap := range snapshots {
		if snap.Path() == path {
			at = i
		}
	}
	if at < 0 {
		return nil, fmt.Errorf("analysis: %s is not a source", path)
	}
	n := len(snapshots)
	files := make([]*syntax.File, n)
	provs := make([]*lower.Provenance, n)
	parallel(n, max(s.cfg.Jobs, 1), func(i int) {
		if i == at {
			files[i], provs[i], _ = lower.Recover(snapshots[i])
		} else {
			files[i], provs[i], _ = lower.Signatures(snapshots[i])
		}
	})
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cp, lib, done, err := s.library()
	if err != nil {
		return nil, err
	}
	defer done()
	rec := &probeRecorder{recorder: newRecorder()}
	_, err = check.CheckOptions(ctx, files, lib, max(s.cfg.Jobs, 1), rec, check.Options{Only: map[*syntax.File]bool{files[at]: true}})
	if err != nil && !errors.Is(err, check.ErrProbed) {
		return nil, err
	}
	out := &Completion{snaps: snapshots}
	v := rec.view
	if v == nil {
		return out, nil
	}
	b := newBuilder(&Result{}, snapshots, files, provs, rec.recorder)
	b.cp = cp
	for i, p := range provs {
		sid := snapshots[i].ID()
		p.Each(func(n any, id psi.NodeID) {
			if _, ok := b.where[n]; !ok {
				b.where[n] = ElementID{sid, id}
			}
		})
	}
	out.Reached = true
	out.Expected = b.cv.typ(v.Expected)
	out.TypePosition, out.Callable, out.Safe = v.TypePosition, v.Callable, v.Safe
	if v.Receiver != nil {
		out.Receiver = b.cv.typ(*v.Receiver)
	}
	if v.StaticOf != nil {
		out.Static = classFQ(v.StaticOf)
	}
	out.Package = v.Package
	for _, t := range v.Receivers {
		if ct := b.cv.typ(t); ct != nil {
			out.Receivers = append(out.Receivers, *ct)
		} else {
			out.Receivers = append(out.Receivers, Type{})
		}
	}
	names := map[string]bool{}
	for _, it := range v.Items {
		ci := b.completionItem(it)
		out.Items = append(out.Items, ci)
		names[ci.Name] = true
	}
	for _, cs := range v.Calls {
		cc := CompletionCall{Argument: cs.Arg, Name: cs.Name, Names: cs.Names}
		for i, f := range cs.Funs {
			var sub types.Subst
			if i < len(cs.Substs) {
				sub = cs.Substs[i]
			}
			cc.Candidates = append(cc.Candidates, b.substCall(f, sub))
		}
		out.Calls = append(out.Calls, cc)
	}
	if first := firstLetter(prefix); first != 0 && !v.Callable && v.Receiver == nil && v.StaticOf == nil && v.Package == "" {
		visible := map[string]bool{}
		for _, ci := range out.Items {
			visible[ci.QualifiedName] = true
		}
		own := ""
		for _, snap := range snapshots {
			if snap.Path() == path {
				own = snapshotPackage(snap)
			}
		}
		consider := func(im check.Importable) {
			fq := qualify(im.Package, im.Name)
			// A class whose name is visible would clash; a function
			// overloads it.
			if im.Default || visible[fq] || im.Class && names[im.Name] || v.TypePosition && !im.Class {
				return
			}
			if r := firstLetter(im.Name); unicode.ToLower(r) == unicode.ToLower(first) && lib.Accessible(im) {
				out.Importables = append(out.Importables, Importable{im.Name, im.Package, im.Class})
			}
		}
		for _, im := range sourceImportables(snapshots) {
			if im.Package != own {
				consider(im)
			}
		}
		for _, im := range lib.Importables() {
			consider(im)
		}
	}
	return out, nil
}

// firstLetter returns the first rune of s if it is a letter, else 0.
func firstLetter(s string) rune {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return r
		}
		return 0
	}
	return 0
}

var origins = [...]string{
	check.Local:         "local",
	check.Member:        "member",
	check.Extension:     "extension",
	check.TopLevel:      "topLevel",
	check.Classifier:    "classifier",
	check.TypeParameter: "typeParameter",
	check.PackageName:   "package",
	check.Static:        "static",
}

// substCall is f's signature, sub applied to its types.
func (b *builder) substCall(f *types.FunSym, sub types.Subst) Call {
	call := b.funCall(f)
	if call.FromJava {
		call.Operator = false // any Java method fits an operator convention
	}
	if sub != nil {
		for i, p := range f.Params {
			if t := b.cv.typ(sub.Apply(p.Type)); t != nil {
				call.Parameters[i] = *t
			}
		}
		call.DeclaredReturnType = b.cv.typ(sub.Apply(f.Ret))
	}
	return call
}

// completionItem converts it.
func (b *builder) completionItem(it check.Item) CompletionItem {
	ci := CompletionItem{Name: it.Name, Origin: origins[it.Origin], Receiver: it.Receiver, Deprecated: it.Deprecated}
	switch {
	case it.Var != nil:
		v := it.Var
		switch v.Kind {
		case types.Local:
			ci.Kind = "local"
		case types.Param:
			ci.Kind = "parameter"
		default:
			ci.Kind = "property"
		}
		ci.QualifiedName = varFQ(v)
		ci.Declaration = b.decl(v.Decl)
	case it.Fun != nil:
		f := it.Fun
		call := b.substCall(f, it.Subst)
		ci.Call = &call
		ci.Kind = "function"
		if it.Prop {
			ci.Kind = "property"
		}
		ci.QualifiedName = call.Name
		if it.Prop {
			ci.QualifiedName = strings.TrimSuffix(call.Name, f.Name) + it.Name
		}
		ci.Declaration = call.Declaration
	case it.Class != nil:
		c := it.Class
		switch {
		case c.Annotation:
			ci.Kind = "annotation"
		case c.Interface:
			ci.Kind = "interface"
		case c.Entries != nil:
			ci.Kind = "enum"
		case c.Object:
			ci.Kind = "object"
		default:
			ci.Kind = "class"
		}
		ci.QualifiedName = classFQ(c)
		ci.Declaration = b.decl(c.Decl)
		if c.Ctor != nil && !c.Interface && !c.Object && !c.Abstract {
			call := b.funCall(c.Ctor)
			ci.Call = &call
		}
	case it.Alias != nil:
		a := it.Alias
		ci.Kind = "typealias"
		ci.QualifiedName = qualify(a.Pkg, a.Name)
		ci.Declaration = b.decl(a.Decl)
	case it.Param != nil:
		ci.Kind = "typeParameter"
		switch o := it.Param.Owner.(type) {
		case *types.ClassSym:
			ci.Declaration = b.decl(o.Decl)
		case *types.FunSym:
			ci.Declaration = b.decl(o.Decl)
		}
	case it.Origin == check.Classifier:
		// A built-in type: Int, String.
		ci.Kind = "class"
		if t := b.cv.typ(it.Type); t != nil {
			ci.QualifiedName = t.Name
		}
	default:
		ci.Kind = "package"
		ci.QualifiedName = it.Package
	}
	if it.Type.K != types.Invalid {
		ci.Type = b.cv.typ(it.Type)
	}
	return ci
}

// PackageMembers returns what an import or package directive may name
// after qualifier, a dotted package name, "" for none: its subpackages
// (Kind "package"), and its classes and top-level functions and
// properties ("class", "function"), of the sources, snapshots, and of the
// class path outside the default imports.
func (s *Session) PackageMembers(snapshots []*psi.Snapshot, qualifier string) ([]CompletionItem, error) {
	cp, lib, done, err := s.library()
	if err != nil {
		return nil, err
	}
	defer done()
	var out []CompletionItem
	seen := map[string]bool{}
	add := func(name, kind string) {
		if name == "" || seen[kind+" "+name] {
			return
		}
		seen[kind+" "+name] = true
		out = append(out, CompletionItem{Name: name, Kind: kind, Origin: "package", QualifiedName: qualify(qualifier, name), Receiver: -1})
	}
	sub := func(pkg string) {
		rest, ok := pkg, qualifier == ""
		if !ok {
			rest, ok = strings.CutPrefix(pkg, qualifier+".")
		}
		if ok && rest != "" {
			first, _, _ := strings.Cut(rest, ".")
			add(first, "package")
		}
	}
	for _, snap := range snapshots {
		pkg := snapshotPackage(snap)
		sub(pkg)
		if pkg != qualifier {
			continue
		}
		for c := snap.RootNode().FirstChild(); c.Valid(); c = c.NextSibling() {
			switch c.NumericKind() {
			case psi.KindCLASS, psi.KindOBJECT_DECLARATION, psi.KindTYPEALIAS:
				add(c.Name(), "class")
			case psi.KindFUN, psi.KindPROPERTY:
				if !c.ReceiverType().Valid() {
					add(c.Name(), "function")
				}
			}
		}
	}
	if cp != nil {
		for _, pkg := range cp.Packages() {
			sub(strings.ReplaceAll(pkg, "/", "."))
		}
	}
	for _, im := range lib.Importables() {
		if im.Package != qualifier || seen["class "+im.Name] && !im.Class || !lib.Accessible(im) {
			continue
		}
		if im.Class {
			add(im.Name, "class")
		} else {
			add(im.Name, "function")
		}
	}
	return out, nil
}

// sourceImportables returns the top-level declarations of snapshots that
// code in another package imports: those not private, and functions and
// properties that aren't extensions.
func sourceImportables(snapshots []*psi.Snapshot) []check.Importable {
	var out []check.Importable
	for _, snap := range snapshots {
		pkg := snapshotPackage(snap)
		for c := snap.RootNode().FirstChild(); c.Valid(); c = c.NextSibling() {
			class := false
			switch c.NumericKind() {
			case psi.KindCLASS, psi.KindOBJECT_DECLARATION, psi.KindTYPEALIAS:
				class = true
			case psi.KindFUN, psi.KindPROPERTY:
				if c.ReceiverType().Valid() {
					continue
				}
			default:
				continue
			}
			if c.Name() == "" || c.ChildKind(psi.KindMODIFIER_LIST).ChildKind(psi.KindPRIVATE_KEYWORD).Valid() {
				continue
			}
			out = append(out, check.Importable{Name: c.Name(), Package: pkg, Class: class})
		}
	}
	return out
}

// snapshotPackage returns the dotted name of snap's package.
func snapshotPackage(snap *psi.Snapshot) string {
	for c := snap.RootNode().FirstChild(); c.Valid(); c = c.NextSibling() {
		if c.NumericKind() == psi.KindPACKAGE_DIRECTIVE {
			name := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(c.Text()), "package"))
			return strings.Join(strings.Fields(name), "")
		}
	}
	return ""
}

// Importables returns what an import may bring into the file at path of
// snapshots under name: the library's and the other packages' sources'
// public classes, typealiases, functions and properties of that name,
// outside the default imports' packages. An unresolved reference's fix
// offers to import them.
func (s *Session) Importables(snapshots []*psi.Snapshot, path, name string) ([]Importable, error) {
	_, lib, done, err := s.library()
	if err != nil {
		return nil, err
	}
	defer done()
	own := ""
	for _, snap := range snapshots {
		if snap.Path() == path {
			own = snapshotPackage(snap)
		}
	}
	var out []Importable
	seen := map[Importable]bool{}
	add := func(im check.Importable) {
		e := Importable{im.Name, im.Package, im.Class}
		if im.Name == name && !im.Default && !seen[e] && lib.Accessible(im) {
			seen[e] = true
			out = append(out, e)
		}
	}
	for _, im := range sourceImportables(snapshots) {
		if im.Package != own {
			add(im)
		}
	}
	all := lib.Importables()
	i, _ := slices.BinarySearchFunc(all, name, func(im check.Importable, n string) int { return strings.Compare(im.Name, n) })
	for ; i < len(all) && all[i].Name == name; i++ {
		add(all[i])
	}
	return out, nil
}

// LibraryImportables calls f with each top-level declaration of the
// library that a file can import or use unimported, in name order, until
// f returns false: the classes and typealiases, functions and properties
// of the class path's packages, but inaccessible classes.
func (s *Session) LibraryImportables(f func(Importable) bool) error {
	_, lib, done, err := s.library()
	if err != nil {
		return err
	}
	defer done()
	for _, im := range lib.Importables() {
		if lib.Accessible(im) && !f(Importable{im.Name, im.Package, im.Class}) {
			return nil
		}
	}
	return nil
}

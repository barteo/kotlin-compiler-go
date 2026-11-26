package lsp

import (
	"context"
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// Check an intention against an overlay. The active index and the client's
// document still represent the source the edits will be applied to.
func (i *index) checkIntention(ctx context.Context, s *psi.Snapshot, edits []edit, verify func(*index, movedDocument) bool) bool {
	preview, err := previewDocument(s, s.Path(), edits)
	if err != nil || len(preview.snap.Diagnostics()) != 0 {
		return false
	}
	next, err := i.workspace.buildIndex(ctx, s.Path(), preview.snap)
	if err != nil {
		return false
	}
	defer next.model.Close()
	if !verify(next, preview) {
		return false
	}

	// An unrelated error elsewhere in the file need not block a local edit.
	prior := map[string]int{}
	for _, d := range i.model.Facts.Diagnostics {
		if !d.Warning && d.Path == s.Path() {
			prior[d.Message]++
		}
	}
	for _, d := range next.model.Facts.Diagnostics {
		if !d.Warning && d.Path == s.Path() {
			prior[d.Message]--
			if prior[d.Message] < 0 {
				return false
			}
		}
	}
	return true
}

func (i *index) intentions(ctx context.Context, at psi.NodeRef, r span) []quickFix {
	var out []quickFix
	decl := ancestor(at, psi.KindPROPERTY, psi.KindFUN)
	if decl.Valid() && !decl.ReturnType().Valid() {
		name := decl.NameIdentifier()
		end := name
		title := "Specify type explicitly"
		if decl.NumericKind() == psi.KindFUN {
			end = decl.ChildKind(psi.KindVALUE_PARAMETER_LIST)
			title = "Specify return type explicitly"
		}
		inferred := decl.NumericKind() == psi.KindPROPERTY || decl.ChildKind(psi.KindEQ).Valid()
		if name.Valid() && end.Valid() && inferred && !before(rng(end).End, r.End) {
			if edits := i.explicitType(ctx, decl, end); len(edits) > 0 {
				out = append(out, quickFix{title, edits})
			}
		}
	}
	call := ancestor(at, psi.KindCALL_EXPRESSION)
	if call.Valid() && !before(rng(call).End, r.End) && !under(at, call.ChildKind(psi.KindLAMBDA_ARGUMENT)) {
		if edits := i.namedArguments(ctx, call); len(edits) > 0 {
			out = append(out, quickFix{"Add names to call arguments", edits})
		}
	}
	return out
}

func (i *index) explicitType(ctx context.Context, decl, end psi.NodeRef) []edit {
	name := decl.NameIdentifier()
	typ := i.symbols[name].Type
	if typ.Name == "" || typ.Platform {
		return nil
	}
	scope := importsAt(decl)
	var compact, qualified strings.Builder
	typeText(&typ, func(value, classifier string) {
		full := value
		if classifier != "" {
			full = dottedName(classifier)
			if alias := scope.aliases[full]; alias != "" {
				value = alias
			}
		}
		compact.WriteString(value)
		qualified.WriteString(full)
	})
	at := rng(end).End
	for _, spelling := range slices.Compact([]string{compact.String(), qualified.String()}) {
		edits := []edit{{span{at, at}, ": " + spelling}}
		if i.checkIntention(ctx, decl.ContainingFile(), edits, func(next *index, preview movedDocument) bool {
			id := preview.token(name)
			actual := next.facts[id.Parent().ReturnType()].Type
			return sameType(&typ, actual)
		}) {
			return edits
		}
	}
	return nil
}

func (i *index) namedArguments(ctx context.Context, call psi.NodeRef) []edit {
	c := i.facts[call].Call
	if c == nil || !c.StableNames || c.FromJava {
		return nil
	}
	params := i.node(c.Declaration).Parameters()
	var edits []edit
	arg := 0
	list := call.ChildKind(psi.KindVALUE_ARGUMENT_LIST)
	for a := list.FirstChild(); a.Valid(); a = a.NextSibling() {
		if a.NumericKind() != psi.KindVALUE_ARGUMENT {
			continue
		}
		if arg >= len(c.ArgumentMapping) {
			return nil
		}
		j := c.ArgumentMapping[arg]
		arg++
		if j < 0 || j >= len(c.ParameterNames) {
			return nil
		}
		if a.ChildKind(psi.KindVALUE_ARGUMENT_NAME).Valid() {
			continue
		}
		if j < len(c.Varargs) && c.Varargs[j] {
			return nil
		}
		name := c.ParameterNames[j]
		if j < len(params) {
			name = params[j].NameIdentifier().Text()
		}
		if name == "" {
			return nil
		}
		at := rng(a).Start
		edits = append(edits, edit{span{at, at}, name + " = "})
	}
	if len(edits) == 0 {
		return nil
	}
	if !i.checkIntention(ctx, call.ContainingFile(), edits, func(next *index, preview movedDocument) bool {
		id := preview.token(factName(call))
		return i.sameCall(next, preview, c, next.facts[ancestor(id, psi.KindCALL_EXPRESSION)].Call)
	}) {
		return nil
	}
	return edits
}

// Compare a call across editor revisions. Source declarations may shift in
// the edited document. For a library overload compare its stable signature.
func (i *index) sameCall(next *index, preview movedDocument, c, other *analysis.Call) bool {
	if other == nil || other.Name != c.Name || other.JVMDescriptor != c.JVMDescriptor || !slices.Equal(other.ArgumentMapping, c.ArgumentMapping) {
		return false
	}
	if original := i.node(c.Declaration); original.Valid() {
		if original.ContainingFile().Path() == preview.snap.Path() {
			original = ancestor(preview.token(original), original.NumericKind())
		}
		return original.Valid() && next.node(other.Declaration) == original
	}
	return other.Declaration == nil && sameType(other.DeclaredReceiver, c.DeclaredReceiver) &&
		slices.EqualFunc(other.Parameters, c.Parameters, func(a, b analysis.Type) bool { return sameType(&a, &b) })
}

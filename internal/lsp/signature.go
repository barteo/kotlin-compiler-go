package lsp

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"unicode/utf16"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/analysis/ka"
	"github.com/barteo/kotlin-compiler-go/psi"
)

type argument struct {
	name string
	typ  *analysis.Type
}
type signatureParam struct {
	name, typ        string
	optional, vararg bool
}
type overload struct {
	name, ret     string
	src           psi.NodeRef
	params        []signatureParam
	score, active int
}

func libraryOverload(name string, d *ka.Decl, ret string) overload {
	o := overload{name: name, ret: ret}
	for _, p := range d.Params {
		o.params = append(o.params, signatureParam{p.Name, libraryType(p.Type), p.Flags&ka.FlagHasDefault != 0, p.Flags&ka.FlagVararg != 0})
	}
	return o
}

// Rank partial calls without treating a missing expression as a type error.
// A named argument selects its declared parameter even before a call resolves.
func (o overload) match(args []argument, cursor int) (int, int) {
	score, active := 0, 0
	used := make([]bool, len(o.params))
	next := 0
	for j := 0; j < max(len(args), cursor+1); j++ {
		var a argument
		if j < len(args) {
			a = args[j]
		}
		slot := next
		if a.name != "" {
			slot = slices.IndexFunc(o.params, func(p signatureParam) bool { return p.name == a.name })
		}
		if slot < 0 || slot >= len(o.params) {
			if len(o.params) == 0 && len(args) == 0 && cursor == 0 {
				return 0, 0
			}
			return 0, -1
		}
		p := o.params[slot]
		if j == cursor {
			active = slot
		}
		if used[slot] && !p.vararg {
			return 0, -1
		}
		used[slot] = true
		if !p.vararg {
			for next < len(used) && used[next] {
				next++
			}
		}
		if a.typ == nil {
			continue
		}
		if typeName(a.typ) == p.typ || a.typ.Name == p.typ {
			score += 8
		} else if slices.ContainsFunc(a.typ.Supertypes, func(t string) bool { return short(t) == p.typ || t == p.typ }) {
			score += 4
		} else {
			score -= 8
		}
	}
	for j, p := range o.params {
		if !used[j] && !p.optional && !p.vararg {
			score--
		}
	}
	return score, active
}
func (i *index) signature(ctx context.Context, n psi.NodeRef, off int, labelOffsets bool) any {
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

	callee := factName(call)
	qual := call.Parent()
	var receiver *analysis.Type
	if (qual.NumericKind() == psi.KindDOT_QUALIFIED_EXPRESSION || qual.NumericKind() == psi.KindSAFE_ACCESS_EXPRESSION) && factName(qual) == callee {
		receiver = i.facts[qual.FirstChild()].Type
		if receiver == nil && len(n.ContainingFile().Diagnostics()) > 0 {
			// A dangling comma can prevent lowering the entire call. Close this
			// argument list in a temporary revision to recover the receiver type.
			s := n.ContainingFile()
			lr := list.Range()
			patched := psi.ParseDocument(s.Path(), s.Source()[:lr.Start]+"()"+s.Source()[lr.End:])
			if temp, err := i.workspace.buildIndex(ctx, s.Path(), patched); err == nil {
				defer temp.model.Close()
				r := qual.FirstChild().Range()
				for x := patched.NodeAt(r.End - 1); x.Valid() && x.Range().End == r.End; x = x.Parent() {
					if x.Range().Start == r.Start && temp.facts[x].Type != nil {
						receiver = temp.facts[x].Type
					}
				}
			}
		}
		if receiver == nil {
			receiver = &analysis.Type{}
		}
	}
	args := i.signatureArguments(list)
	options := i.signatureOverloads(ctx, callee, receiver)
	resolved := i.facts[call].Call
	var resolvedTypes []string
	if resolved != nil {
		for _, t := range resolved.Parameters {
			resolvedTypes = append(resolvedTypes, typeName(&t))
		}
	}
	haveResolved := false
	for j := range options {
		o := &options[j]
		o.score, o.active = o.match(args, arg)
		if resolved != nil && o.active >= 0 && len(o.params) == len(resolvedTypes) {
			match := true
			for k, p := range o.params {
				if p.typ != resolvedTypes[k] {
					match = false
					break
				}
			}
			if match {
				o.score += 1000
				haveResolved = true
			}
		}
	}
	if resolved != nil && !haveResolved {
		var labels []string
		for j, typ := range resolvedTypes {
			if j < len(resolved.ParameterNames) {
				typ = resolved.ParameterNames[j] + ": " + typ
			}
			labels = append(labels, typ)
		}
		if arg < len(resolved.ArgumentMapping) {
			active = resolved.ArgumentMapping[arg]
		}
		appendSig(callee.Name(), labels, typeName(resolved.ReturnType), i.node(resolved.Declaration), active)
	}
	slices.SortStableFunc(options, func(a, b overload) int { return cmp.Compare(b.score, a.score) })
	for _, o := range options {
		if o.active < 0 {
			continue
		}
		var labels []string
		for _, p := range o.params {
			label := p.name + ": " + p.typ
			if p.vararg {
				label = "vararg " + label
			}
			if p.optional && !o.src.Valid() {
				label += " = COMPILED_CODE"
			}
			labels = append(labels, label)
		}
		appendSig(o.name, labels, o.ret, o.src, o.active)
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

func (i *index) signatureArguments(list psi.NodeRef) []argument {
	var args []argument
	for child := list.FirstChild(); child.Valid(); child = child.NextSibling() {
		if child.NumericKind() != psi.KindVALUE_ARGUMENT {
			continue
		}
		var v argument
		for expr := child.FirstChild(); expr.Valid(); expr = expr.NextSibling() {
			if expr.NumericKind() == psi.KindVALUE_ARGUMENT_NAME {
				v.name = expr.ChildKind(psi.KindREFERENCE_EXPRESSION).Name()
				continue
			}
			if t := i.facts[expr].Type; t != nil {
				v.typ = t
			}
			if v.typ == nil { // The checker can discard an incomplete call before recording its arguments.
				switch expr.NumericKind() {
				case psi.KindINTEGER_CONSTANT:
					v.typ = &analysis.Type{Name: "kotlin.Int"}
				case psi.KindSTRING_TEMPLATE:
					v.typ = &analysis.Type{Name: "kotlin.String"}
				case psi.KindBOOLEAN_CONSTANT:
					v.typ = &analysis.Type{Name: "kotlin.Boolean"}
				}
			}
		}
		args = append(args, v)
	}
	return args
}

func (i *index) signatureOverloads(ctx context.Context, callee psi.NodeRef, receiver *analysis.Type) []overload {
	var options []overload
	for _, c := range i.candidates(ctx, callee, callee.Name(), receiver, false) {
		if c.name != callee.Name() {
			continue
		}
		if c.classID != "" {
			if cl := i.model.Class(c.classID); cl != nil {
				for _, m := range cl.Members {
					if m.Kind == ka.KindConstructor && m.Visibility == ka.Public {
						options = append(options, libraryOverload(c.name, m, cl.Name))
					}
				}
			}
		} else if c.binary != nil && c.binary.Kind == ka.KindFunction {
			options = append(options, libraryOverload(c.name, c.binary, libraryType(c.binary.Type)))
		} else if d := c.source; d.Valid() && d.NumericKind() == psi.KindFUN {
			sym := i.symbols[d.NameIdentifier()]
			o := overload{name: c.name, src: d, ret: typeName(&sym.Type)}
			if o.ret == "" {
				o.ret = d.ReturnType().Text()
			}
			for _, p := range d.Parameters() {
				o.params = append(o.params, signatureParam{p.Name(), p.TypeReference().Text(), p.ChildKind(psi.KindEQ).Valid(), p.ChildKind(psi.KindMODIFIER_LIST).ChildKind(psi.KindVARARG_KEYWORD).Valid()})
			}
			options = append(options, o)
		}
	}
	return options
}

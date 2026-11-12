package ide

import (
	"cmp"
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// SignatureContext is a caret in a call's arguments, for signature help.
type SignatureContext struct {
	Path string
	// Argument is the index of the argument at the caret, and Name its
	// name if it is a named argument.
	Argument int
	Name     string
	// ProbeText is the document with analysis.ProbeName as the argument
	// at the caret, and the call's ) if it is missing.
	ProbeText string

	at, delta int // where ProbeText differs, and how much longer it is
}

// SignatureAt returns the call argument the caret at offset off of d is
// in, if it is in one: in a call's parentheses, not in a lambda there.
func SignatureAt(d *Document, off int) (SignatureContext, bool) {
	text := d.Text()
	if off <= 0 || off > len(text) || inCommentOrString(d.Snap, off) {
		return SignatureContext{}, false
	}
	var list psi.NodeRef
	for n := d.Snap.NodeAt(off - 1); n.Valid(); n = n.Parent() {
		k := n.NumericKind()
		if k == psi.KindFUNCTION_LITERAL || k == psi.KindBLOCK || k == psi.KindCLASS_BODY {
			return SignatureContext{}, false
		}
		if k != psi.KindVALUE_ARGUMENT_LIST {
			continue
		}
		lpar, rpar := n.ChildKind(psi.KindLPAR), n.ChildKind(psi.KindRPAR)
		if lpar.Valid() && off > lpar.Range().Start && (!rpar.Valid() || off <= rpar.Range().Start) {
			list = n
			break
		}
	}
	if !list.Valid() {
		return SignatureContext{}, false
	}
	sc := SignatureContext{Path: d.Path}
	// The argument at the caret, or the one before it when no comma
	// follows that.
	var arg psi.NodeRef
	for c := list.FirstChild(); c.Valid(); c = c.NextSibling() {
		r := c.Range()
		switch c.NumericKind() {
		case psi.KindCOMMA:
			if r.Start < off {
				sc.Argument++
				arg = psi.NodeRef{}
			}
		case psi.KindVALUE_ARGUMENT:
			if r.Start <= off {
				arg = c
			}
		}
	}
	start, end := off, off // the text the probe replaces
	if arg.Valid() {
		start, end = arg.Range().Start, arg.Range().End
		if name := arg.ChildKind(psi.KindVALUE_ARGUMENT_NAME); name.Valid() {
			if eq := arg.ChildKind(psi.KindEQ); eq.Valid() && eq.Range().End <= off {
				sc.Name = strings.TrimSpace(name.Text())
				start = eq.Range().End
			}
		}
	}
	probe := " " + analysis.ProbeName
	if !list.ChildKind(psi.KindRPAR).Valid() {
		probe += ")"
	}
	probe += " "
	sc.ProbeText = text[:start] + probe + text[end:]
	sc.at, sc.delta = start, len(probe)-(end-start)
	return sc, true
}

// Signature is a candidate of a call, for signature help.
type Signature struct {
	Label string
	// Parameters are the parameters' [start, end) in Label, in bytes.
	Parameters [][2]int
	// Active is the index of the parameter the caret's argument is for,
	// or -1.
	Active int
	Decl   *DeclAt
}

// SignatureHelp returns the candidates of the call at sc from c, and the
// index of the one that fits the arguments so far.
func SignatureHelp(sc SignatureContext, c *analysis.Completion) (sigs []Signature, active int) {
	if c == nil || len(c.Calls) == 0 {
		return nil, 0
	}
	cs := c.Calls[0]
	// Fewest parameters first; the first that fits is the active one.
	cands := slices.Clone(cs.Candidates)
	slices.SortStableFunc(cands, func(a, b analysis.Call) int { return cmp.Compare(len(a.Parameters), len(b.Parameters)) })
	active = -1
	for i, cand := range cands {
		label, params := renderSignature(cand)
		p := cs.Argument
		if cs.Name != "" {
			p = slices.Index(cand.ParameterNames, cs.Name)
		} else if n := len(cand.Parameters); p >= n && n > 0 && n <= len(cand.Varargs) && cand.Varargs[n-1] {
			p = n - 1
		}
		if p >= len(cand.Parameters) {
			p = -1
		}
		fits := p >= 0 && !slices.ContainsFunc(cs.Names, func(name string) bool { return !slices.Contains(cand.ParameterNames, name) })
		if active < 0 && fits || active < 0 && i == len(cands)-1 {
			active = i
		}
		sig := Signature{Label: label, Parameters: params, Active: p}
		if cand.Declaration != nil {
			if n := c.Node(*cand.Declaration); n.Valid() {
				path, off := n.ContainingFile().Path(), n.Range().Start
				if path == sc.Path && off > sc.at {
					off -= sc.delta
				}
				sig.Decl = &DeclAt{Path: path, Off: off}
			}
		}
		sigs = append(sigs, sig)
	}
	return sigs, max(active, 0)
}

// SignatureDoc returns the KDoc of the declaration starting at offset off
// of snap, as Markdown, or "".
func SignatureDoc(snap *psi.Snapshot, off int) string {
	if snap == nil || off < 0 || off >= len(snap.Source()) {
		return ""
	}
	for n := snap.NodeAt(off); n.Valid() && n.Range().Start == off; n = n.Parent() {
		if n.Declaration().Valid() {
			return KDoc(n)
		}
	}
	return ""
}

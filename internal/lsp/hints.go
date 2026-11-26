package lsp

import (
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// These defaults and option names follow the Kotlin editor extension.
type hintSettings struct {
	property, variable, returns, parameterType, arguments, compiled bool
	lambdaReturns, lambdaParameters, chains                         bool
}

func readHintSettings(options object) hintSettings {
	h := hintSettings{property: true, variable: true, returns: true, parameterType: true,
		arguments: true, compiled: true, lambdaReturns: true, lambdaParameters: true}
	fields := map[string]*bool{
		"settings.types.property": &h.property, "settings.types.variable": &h.variable,
		"type.function.return": &h.returns, "type.function.parameter": &h.parameterType,
		"parameters": &h.arguments, "parameters.compiled": &h.compiled,
		"settings.lambda.return":      &h.lambdaReturns,
		"lambda.receivers.parameters": &h.lambdaParameters, "call.chains": &h.chains,
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
		if before(at, r.Start) || !before(at, r.End) {
			return
		}
		hint := object{"position": at, "label": label}
		if kind == 0 {
			hint["paddingLeft"] = true
		} else {
			hint["kind"] = kind
		}
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
		case psi.KindLAMBDA_EXPRESSION:
			lambdaHints(n, i, h, add)
		case psi.KindDOT_QUALIFIED_EXPRESSION, psi.KindSAFE_ACCESS_EXPRESSION:
			if h.chains && !chainReceiver(n) {
				chainHints(n, i, add)
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

type addHint func(position, string, int)

func lambdaHints(n psi.NodeRef, i *index, h hintSettings, add addHint) {
	f := i.facts[n]
	literal := n.ChildKind(psi.KindFUNCTION_LITERAL)
	brace := literal.ChildKind(psi.KindLBRACE)
	if h.lambdaParameters && newlineAfter(brace) {
		var labels []string
		if t := typeName(f.LambdaReceiver); t != "" {
			labels = append(labels, "this: "+t)
		}
		if f.ImplicitParameter && len(f.LambdaParameters) == 1 {
			id := f.LambdaParameters[0]
			if id > 0 && int(id) <= len(i.model.Facts.Symbols) {
				t := &i.model.Facts.Symbols[id-1].Type
				if name := typeName(t); name != "" && t.Name != "kotlin.Unit" {
					labels = append(labels, "it: "+name)
				}
			}
		}
		if len(labels) > 0 {
			at := rng(brace).End
			add(at, " ", 1)
			for _, label := range labels {
				add(at, label, 1)
				add(at, " ", 1)
			}
		}
	}
	if !h.lambdaReturns || f.Type == nil || len(f.Type.Arguments) == 0 {
		return
	}
	result := f.Type.Arguments[len(f.Type.Arguments)-1]
	if result.Name == "" || result.Name == "kotlin.Unit" {
		return
	}
	block := literal.ChildKind(psi.KindBLOCK)
	tail, count := lastExpression(block)
	if !tail.Valid() || rng(n).Start.Line == rng(n).End.Line {
		return
	}
	// A lambda with a single simple expression already shows its result.
	if count == 1 && tail.NumericKind() != psi.KindIF && tail.NumericKind() != psi.KindWHEN {
		return
	}
	name := "lambda"
	parent := n.Parent()
	if parent.NumericKind() == psi.KindLABELED_EXPRESSION {
		name = strings.TrimSuffix(parent.ChildKind(psi.KindLABEL_QUALIFIER).Text(), "@")
	} else if parent.NumericKind() == psi.KindLAMBDA_ARGUMENT {
		if callee := factName(parent.Parent()); callee.Valid() {
			name = callee.Name()
		}
	}
	lambdaResultHints(tail, i, "^"+name, add)
}

func lastExpression(n psi.NodeRef) (last psi.NodeRef, count int) {
	for c := n.FirstChild(); c.Valid(); c = c.NextSibling() {
		if !c.IsLeaf() {
			last = c
			count++
		}
	}
	return
}

func lambdaResultHints(n psi.NodeRef, i *index, label string, add addHint) {
	switch n.NumericKind() {
	case psi.KindIF, psi.KindWHEN:
		if rng(n).Start.Line != rng(n).End.Line {
			for c := n.FirstChild(); c.Valid(); c = c.NextSibling() {
				switch c.NumericKind() {
				case psi.KindTHEN, psi.KindELSE, psi.KindWHEN_ENTRY:
					tail, _ := lastExpression(c)
					lambdaResultHints(tail, i, label, add)
				}
			}
			return
		}
	case psi.KindBLOCK:
		tail, _ := lastExpression(n)
		lambdaResultHints(tail, i, label, add)
		return
	case psi.KindRETURN, psi.KindTHROW:
		return
	}
	f := i.facts[n]
	if f.UsedAsExpression && f.Type != nil && f.Type.Name != "" && f.Type.Name != "kotlin.Nothing" {
		add(rng(n).End, label, 1)
	}
}

func newlineAfter(n psi.NodeRef) bool {
	ws := n.NextSibling()
	return ws.NumericKind() == psi.KindWHITE_SPACE && strings.Contains(ws.Text(), "\n")
}

func qualified(n psi.NodeRef) bool {
	return n.NumericKind() == psi.KindDOT_QUALIFIED_EXPRESSION || n.NumericKind() == psi.KindSAFE_ACCESS_EXPRESSION
}

// A parenthesized receiver, indexing operation or postfix assertion may
// sit between two segments of the same qualified expression.
func chainOperand(n psi.NodeRef) psi.NodeRef {
	switch n.NumericKind() {
	case psi.KindPARENTHESIZED, psi.KindPOSTFIX_EXPRESSION, psi.KindARRAY_ACCESS_EXPRESSION, psi.KindCALL_EXPRESSION:
		for c := n.FirstChild(); c.Valid(); c = c.NextSibling() {
			if !c.IsLeaf() {
				return c
			}
		}
	}
	return psi.NodeRef{}
}

func chainReceiver(n psi.NodeRef) bool {
	for p := n.Parent(); chainOperand(p) == n; p = n.Parent() {
		n = p
	}
	return qualified(n.Parent()) && n.Parent().FirstChild() == n
}

func chainSegment(n psi.NodeRef) psi.NodeRef {
	for {
		child := chainOperand(n)
		if !child.Valid() {
			return n
		}
		n = child
	}
}

func chainHints(n psi.NodeRef, i *index, add addHint) {
	// Only receivers followed by a line break can carry a hint. Walk from
	// the base of the chain to suppress consecutive identical result types.
	var receivers []psi.NodeRef
	for segment := n; qualified(segment); segment = chainSegment(n) {
		n = segment.FirstChild()
		if newlineAfter(n) {
			receivers = append(receivers, n)
		}
	}
	slices.Reverse(receivers)
	var points []psi.NodeRef
	var first, previous *analysis.Type
	changed := false
	for j, expr := range receivers {
		t := i.facts[expr].Type
		if t == nil || t.Name == "" {
			return
		}
		if (j == 0 && qualified(chainSegment(expr))) || (j > 0 && (!qualified(chainSegment(receivers[j-1])) || !sameType(t, previous))) {
			points = append(points, expr)
			if first == nil {
				first = t
			} else if !sameType(first, t) {
				changed = true
			}
		}
		previous = t
	}
	if changed {
		for _, expr := range points {
			add(rng(expr).End, " "+typeName(i.facts[expr].Type), 0)
		}
	}
}

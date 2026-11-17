package ide

import (
	"strings"

	"github.com/barteo/kotlin-compiler-go/psi"
)

// Style inspections: what IntelliJ grays out as redundant, judged by the
// syntax alone. Each is a finding with its own edit, so that its quick
// fix finds it again at the diagnostic's range (findingFix).

// Codes of the style inspections.
const (
	RedundantSemicolon            = "REDUNDANT_SEMICOLON"
	RedundantUnitReturnType       = "REDUNDANT_UNIT_RETURN_TYPE"
	RedundantVisibilityModifier   = "REDUNDANT_VISIBILITY_MODIFIER"
	RedundantModalityModifier     = "REDUNDANT_MODALITY_MODIFIER"
	RemoveEmptyClassBody          = "REMOVE_EMPTY_CLASS_BODY"
	RemoveEmptyPrimaryConstructor = "REMOVE_EMPTY_PRIMARY_CONSTRUCTOR"
	RedundantConstructorKeyword   = "REDUNDANT_CONSTRUCTOR_KEYWORD"
	RemoveEmptyLambdaParentheses  = "REMOVE_EMPTY_PARENTHESES_FROM_LAMBDA_CALL"
	RemoveTemplateBraces          = "REMOVE_CURLY_BRACES_FROM_TEMPLATE"
)

// A finding is an inspection's warning and the edit of its fix: Edit's
// range replaced with Text.
type finding struct {
	Diagnostic
	Title string
	Edit  psi.Range
	Text  string
}

// findingCodes are the codes whose fixes are their findings' edits.
var findingCodes = map[string]bool{
	RedundantSemicolon: true, RedundantUnitReturnType: true, RedundantVisibilityModifier: true,
	RedundantModalityModifier: true, RemoveEmptyClassBody: true, RemoveEmptyPrimaryConstructor: true,
	RedundantConstructorKeyword: true, RemoveEmptyLambdaParentheses: true, RemoveTemplateBraces: true,
}

// styleFindings returns the style findings of snap; explicitAPI is the
// module's explicit API mode, under which public isn't redundant.
func styleFindings(snap *psi.Snapshot, explicitAPI string) []finding {
	src := snap.Source()
	var out []finding
	redundant := func(r psi.Range, code, msg, title string, edit psi.Range, text string) {
		out = append(out, finding{Diagnostic{Range: r, Severity: Hint, Code: code, Message: msg, Unnecessary: true}, title, edit, text})
	}
	remove := func(n psi.NodeRef, code, msg, title string) {
		redundant(n.Range(), code, msg, title, withSpaceBefore(src, n.Range()), "")
	}
	psi.WalkNodes(snap.RootNode(), func(n psi.NodeRef) bool {
		switch n.NumericKind() {
		case psi.KindSEMICOLON:
			if redundantSemicolon(src, n) {
				redundant(n.Range(), RedundantSemicolon, "Redundant semicolon", "Remove redundant semicolon", n.Range(), "")
			}
		case psi.KindFUN:
			if t := n.TypeReference(); t.Valid() && n.Body().NumericKind() == psi.KindBLOCK && !n.ChildKind(psi.KindEQ).Valid() &&
				!hasModifier(n, psi.KindOVERRIDE_KEYWORD) {
				if txt := compact(t.Text()); txt == "Unit" || txt == "kotlin.Unit" {
					params := n.ChildKind(psi.KindVALUE_PARAMETER_LIST)
					if params.Valid() && params.Range().End < t.Range().Start {
						redundant(t.Range(), RedundantUnitReturnType, "Redundant 'Unit' return type", "Remove redundant 'Unit' return type",
							psi.Range{Start: params.Range().End, End: t.Range().End}, "")
					}
				}
			}
		case psi.KindMODIFIER_LIST:
			decl := n.Parent()
			if hasAnnotation(decl) || hasAnnotation(containingClass(decl)) {
				break
			}
			if pub := n.ChildKind(psi.KindPUBLIC_KEYWORD); pub.Valid() && explicitAPI != "strict" && explicitAPI != "warning" &&
				!n.ChildKind(psi.KindOVERRIDE_KEYWORD).Valid() && decl.NumericKind() != psi.KindPROPERTY_ACCESSOR {
				remove(pub, RedundantVisibilityModifier, "Redundant visibility modifier", "Remove redundant 'public' modifier")
			}
			if m, ok := redundantModality(decl, n); ok {
				remove(m, RedundantModalityModifier, "Redundant modality modifier", "Remove redundant '"+m.Text()+"' modifier")
			}
		case psi.KindCLASS_BODY:
			if d := n.Parent(); emptyBody(n) && d.NumericKind() != psi.KindOBJECT_LITERAL && d.Parent().NumericKind() != psi.KindOBJECT_LITERAL &&
				!hasModifier(d, psi.KindENUM_KEYWORD) {
				remove(n, RemoveEmptyClassBody, "Redundant empty class body", "Remove empty class body")
			}
		case psi.KindPRIMARY_CONSTRUCTOR:
			params := n.ChildKind(psi.KindVALUE_PARAMETER_LIST)
			if n.ChildKind(psi.KindMODIFIER_LIST).Valid() || !params.Valid() {
				break
			}
			if kw := n.ChildKind(psi.KindCONSTRUCTOR_KEYWORD); kw.Valid() {
				redundant(kw.Range(), RedundantConstructorKeyword, "Redundant 'constructor' keyword", "Remove redundant 'constructor' keyword",
					withSpaceBefore(src, psi.Range{Start: kw.Range().Start, End: params.Range().Start}), "")
			} else if len(n.Parameters()) == 0 && !strings.Contains(params.Text(), "/") && !secondaryConstructors(n.Parent()) &&
				!hasModifier(n.Parent(), psi.KindEXPECT_KEYWORD) && !hasModifier(n.Parent(), psi.KindDATA_KEYWORD) {
				redundant(n.Range(), RemoveEmptyPrimaryConstructor, "Remove empty primary constructor", "Remove empty primary constructor", n.Range(), "")
			}
		case psi.KindVALUE_ARGUMENT_LIST:
			if n.Parent().NumericKind() == psi.KindCALL_EXPRESSION && compact(n.Text()) == "()" {
				if next := n.NextSibling(); next.NumericKind() == psi.KindLAMBDA_ARGUMENT ||
					trivia(next) && !strings.Contains(next.Text(), "\n") && next.NextSibling().NumericKind() == psi.KindLAMBDA_ARGUMENT {
					redundant(n.Range(), RemoveEmptyLambdaParentheses, "Unnecessary parentheses in function call with lambda",
						"Remove unnecessary parentheses from function call with lambda", n.Range(), "")
				}
			}
		case psi.KindLONG_STRING_TEMPLATE_ENTRY:
			if name, ok := braceless(src, n); ok {
				redundant(n.Range(), RemoveTemplateBraces, "Redundant curly braces in string template", "Remove curly braces", n.Range(), "$"+name)
			}
		}
		return true
	})
	return out
}

// withSpaceBefore extends r over the blanks before it on its line, if
// what follows r on the line isn't a blank, so that removing it doesn't
// leave two spaces.
func withSpaceBefore(src string, r psi.Range) psi.Range {
	start := r.Start
	for start > 0 && (src[start-1] == ' ' || src[start-1] == '\t') {
		start--
	}
	if start > 0 && src[start-1] != '\n' {
		return psi.Range{Start: start, End: r.End}
	}
	if r.End < len(src) && (src[r.End] == ' ' || src[r.End] == '\t') {
		end := r.End
		for end < len(src) && (src[end] == ' ' || src[end] == '\t') {
			end++
		}
		return psi.Range{Start: r.Start, End: end}
	}
	return r
}

// redundantSemicolon reports whether a semicolon ends its line's last
// statement, with nothing on the next line that could join the statement
// before it without it: a lambda, a parenthesis or bracket, a call or an
// operator that may start a line.
func redundantSemicolon(src string, n psi.NodeRef) bool {
	if p := n.Parent(); p.NumericKind() == psi.KindCLASS_BODY && hasModifier(p.Parent(), psi.KindENUM_KEYWORD) {
		return false // ends an enum's empty entry list
	} else if p.NumericKind() == psi.KindENUM_ENTRY {
		for d := p.NextSibling(); d.Valid(); d = d.NextSibling() {
			if !trivia(d) && d.NumericKind() != psi.KindRBRACE {
				return false // members follow the entries
			}
		}
	}
	var prev psi.NodeRef
	for c := n.Parent().FirstChild(); c.Valid() && c.Range().Start < n.Range().Start; c = c.NextSibling() {
		if !trivia(c) {
			prev = c
		}
	}
	if prev.Valid() {
		var last psi.NodeRef
		for c := prev.FirstChild(); c.Valid(); c = c.NextSibling() {
			last = c
		}
		if last.Valid() && last.Range().Start == last.Range().End {
			return false // the empty body of a loop or an if
		}
	}
	end := n.Range().End
	nl := strings.IndexByte(src[end:], '\n')
	rest := src[end:]
	if nl >= 0 {
		rest = src[end : end+nl]
	}
	if r := strings.TrimSpace(rest); strings.HasPrefix(r, "}") {
		return true
	} else if r != "" && !strings.HasPrefix(r, "//") {
		return false
	}
	if nl < 0 {
		return true
	}
	next := strings.TrimLeft(src[end+nl:], " \t\r\n")
	if next == "" {
		return true
	}
	switch next[0] {
	case '{', '(', '[', '.', '?', '&', '|', ':':
		return false
	}
	return !strings.HasPrefix(next, "as ") && !strings.HasPrefix(next, "as?")
}

// redundantModality returns the modifier of a member of mods that says
// what its declaration has anyway: final on a final class's member that
// isn't an override, open on an interface's member, abstract on an
// interface's member without a body.
func redundantModality(decl, mods psi.NodeRef) (psi.NodeRef, bool) {
	cls := containingClass(decl)
	if !cls.Valid() || cls.NumericKind() == psi.KindOBJECT_LITERAL {
		return psi.NodeRef{}, false
	}
	switch decl.NumericKind() {
	case psi.KindFUN, psi.KindPROPERTY:
	default:
		return psi.NodeRef{}, false
	}
	if cls.ChildKind(psi.KindINTERFACE_KEYWORD).Valid() {
		if m := mods.ChildKind(psi.KindOPEN_KEYWORD); m.Valid() {
			return m, true
		}
		if m := mods.ChildKind(psi.KindABSTRACT_KEYWORD); m.Valid() && !decl.Body().Valid() && !decl.ChildKind(psi.KindPROPERTY_ACCESSOR).Valid() && !decl.ChildKind(psi.KindEQ).Valid() {
			return m, true
		}
		return psi.NodeRef{}, false
	}
	m := mods.ChildKind(psi.KindFINAL_KEYWORD)
	if !m.Valid() || mods.ChildKind(psi.KindOVERRIDE_KEYWORD).Valid() {
		return psi.NodeRef{}, false
	}
	if cls.NumericKind() == psi.KindCLASS && (hasModifier(cls, psi.KindOPEN_KEYWORD) || hasModifier(cls, psi.KindABSTRACT_KEYWORD) ||
		hasModifier(cls, psi.KindSEALED_KEYWORD) || hasModifier(cls, psi.KindENUM_KEYWORD)) {
		return psi.NodeRef{}, false
	}
	return m, true
}

// containingClass returns the class, interface or object whose body holds
// decl, or none.
func containingClass(decl psi.NodeRef) psi.NodeRef {
	if body := decl.Parent(); body.NumericKind() == psi.KindCLASS_BODY {
		return body.Parent()
	}
	return psi.NodeRef{}
}

// hasAnnotation reports whether d has an annotation: a plugin, as allopen,
// may give its modifiers meaning.
func hasAnnotation(d psi.NodeRef) bool {
	mods := d.ChildKind(psi.KindMODIFIER_LIST)
	return mods.ChildKind(psi.KindANNOTATION_ENTRY).Valid() || mods.ChildKind(psi.KindANNOTATION).Valid()
}

// emptyBody reports whether a class body holds nothing, not even a
// comment.
func emptyBody(body psi.NodeRef) bool {
	for c := body.FirstChild(); c.Valid(); c = c.NextSibling() {
		switch {
		case c.NumericKind() == psi.KindLBRACE, c.NumericKind() == psi.KindRBRACE:
		case strings.TrimSpace(c.Text()) == "":
		default:
			return false
		}
	}
	return true
}

// secondaryConstructors reports whether a class declares a secondary
// constructor, which would call the primary one.
func secondaryConstructors(cls psi.NodeRef) bool {
	for c := cls.ChildKind(psi.KindCLASS_BODY).FirstChild(); c.Valid(); c = c.NextSibling() {
		if c.NumericKind() == psi.KindSECONDARY_CONSTRUCTOR {
			return true
		}
	}
	return false
}

// braceless returns the name a template entry ${name} can be written
// $name with: a simple name or an unlabeled this, not followed by what
// would continue the name.
func braceless(src string, n psi.NodeRef) (string, bool) {
	var x psi.NodeRef
	for c := n.FirstChild(); c.Valid(); c = c.NextSibling() {
		switch c.NumericKind() {
		case psi.KindLONG_TEMPLATE_ENTRY_START, psi.KindLONG_TEMPLATE_ENTRY_END:
		default:
			if x.Valid() {
				return "", false
			}
			x = c
		}
	}
	name := x.Text()
	switch x.NumericKind() {
	case psi.KindREFERENCE_EXPRESSION:
		if strings.HasPrefix(name, "`") || x.FirstChild().NumericKind() != psi.KindIDENTIFIER {
			return "", false
		}
	case psi.KindTHIS_EXPRESSION:
		if name != "this" {
			return "", false
		}
	default:
		return "", false
	}
	if end := n.Range().End; end < len(src) {
		if c := src[end]; c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 0x80 {
			return "", false
		}
	}
	return name, true
}

// findingFix returns the fix of a finding's diagnostic: the finding of
// its code at its range, found again.
func (f fixer) findingFix(findings []finding) []Fix {
	for _, x := range findings {
		if x.Code == f.d.Code && x.Range == f.d.Range {
			return f.fix(x.Title, true, f.edit(x.Edit, x.Text))
		}
	}
	return nil
}

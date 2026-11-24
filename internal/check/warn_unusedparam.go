package check

import (
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
)

// unusedLambdaParams reports, as kotlinc's extra checker (-Wextra) does,
// UNUSED_ANONYMOUS_PARAMETER of each parameter a lambda names but its
// body never uses: not _, a destructured parameter, nor an anonymous
// function's. kc counts any use of the name in the body as a use of the
// parameter, even where another variable of the name shadows it, so as
// never to warn of a parameter that is used.
func (c *Checker) unusedLambdaParams() {
	// kotlinc counts uses in annotations' arguments, which kc doesn't
	// resolve, so it leaves alone the parameters of those names.
	annotNames := map[string]bool{}
	for _, as := range c.file.Annotated {
		for _, a := range as {
			for _, arg := range a.Args {
				for _, t := range arg.Toks {
					if t.Kind == syntax.IDENT {
						annotNames[t.Text] = true
					}
				}
			}
		}
	}
	walkCode(c.file.Decls, func(n any) bool {
		l, ok := n.(*syntax.Lambda)
		if !ok || l.Ref != nil || l.Anon || l.Body == nil || !l.Arrow {
			return true
		}
		for _, p := range l.Params {
			if p == nil || p.Sym == nil || p.Names != nil || p.P.Line == 0 || p.Name == "" ||
				(p.Name == "_" && !p.Quoted) || annotNames[p.Name] {
				continue
			}
			used := false
			walkCode([]syntax.Decl{&syntax.FunDecl{Body: l.Body}}, func(m any) bool {
				if id, ok := m.(*syntax.Ident); ok && id.Name == p.Name {
					used = true
				}
				return !used
			})
			if !used {
				c.warnf(p.P, "UNUSED_ANONYMOUS_PARAMETER", "parameter '%s: %s' is never used, consider renaming it to '_'.", p.Name, p.Sym.Type)
			}
		}
		return true
	})
}

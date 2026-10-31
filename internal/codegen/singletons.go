package codegen

import (
	"path/filepath"
	"strconv"
	"strings"

	cf "github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/kmeta"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Composable singletons.
//
// As kotlinc's Compose plugin, kc creates the object of a composable
// lambda that returns Unit and captures nothing once, in a static field
// of the file's ComposableSingletons$FileKt object: its static
// initializer wraps it in a ComposableLambda, composableLambdaInstance(key,
// false, lambda), and code gets it through the field's internal getter:
//
//	ComposableSingletons$FileKt.INSTANCE.getLambda$key$module()
//
// The lambda's method is a method of that class, which calls private
// members of the file's classes through their accessors.

// composeSingletons is the file's ComposableSingletons class, being
// generated: its static initializer's code, which is completed last, and
// the getters of the lambdas it holds.
type composeSingletons struct {
	cls     *cf.Class
	clinit  *cf.Code
	getters map[*syntax.Lambda]string
	names   map[string]bool
	compat  int // the getLambda-N getters: see compatSingleton
}

// singleton reports whether composable lambda x, created by the code
// being generated, is the file's ComposableSingletons class's (see
// syntax.Lambda.Singleton), in code that other modules don't copy.
func (g *gen) singleton(x *syntax.Lambda) bool {
	return x.Singleton() && !g.publicInline && !g.declaredInCopy(x)
}

// compatSingleton adds composable lambda x, of the code of a public
// inline function, to the file's ComposableSingletons class, if it would
// be one of its lambdas elsewhere, as kotlinc's plugin does, though the
// function's code creates an object of a class of its own: with a second
// getter, getLambda-N$module, N counting those of the file's inline
// functions, nested ones first.
func (g *gen) compatSingleton(x *syntax.Lambda) {
	if !g.publicInline || !x.Singleton() || g.declaredInCopy(x) {
		return
	}
	s := g.composeSingletons()
	if _, ok := s.getters[x]; !ok {
		saved := g.inlineSingletons
		g.inlineSingletons = true
		g.addSingleton(s, x)
		g.inlineSingletons = saved
	}
}

// singletonLambda pushes composable lambda x, the file's ComposableSingletons
// class's (see singleton), creating its field and getter, and generating
// its method, the first time.
func (g *gen) singletonLambda(x *syntax.Lambda) {
	s := g.composeSingletons()
	getter, ok := s.getters[x]
	if !ok {
		getter = g.addSingleton(s, x)
	}
	fd := "L" + types.ClassName(types.Erase(x.T)) + ";"
	g.code.FieldInsn(cf.GETSTATIC, s.cls.Name, "INSTANCE", "L"+s.cls.Name+";")
	g.code.Invoke(cf.INVOKEVIRTUAL, s.cls.Name, getter, "()"+fd)
}

// addSingleton adds composable lambda x to the file's ComposableSingletons
// class s: its field, which the static initializer assigns, and the
// field's getter, whose name it returns.
func (g *gen) addSingleton(s *composeSingletons, x *syntax.Lambda) string {
	key := strconv.Itoa(int(g.lambdaKey(x)))
	name := "lambda$" + key
	for i := 0; s.names[name]; i++ {
		name = "lambda$" + key + "$" + strconv.Itoa(i)
	}
	s.names[name] = true
	fd := "L" + types.ClassName(types.Erase(x.T)) + ";"
	s.cls.AddField(cf.AccPrivate|cf.AccStatic, name, fd)

	// The object, created in the static initializer: the lambda's method
	// is the class's, kotlinc's lambda_key$lambda$0.
	sg := &gen{file: g.file, cls: s.cls, code: s.clinit, shared: g.shared, noLines: g.noLines,
		prefix: "lambda_" + strings.ReplaceAll(key, "-", "_") + "$lambda", inlineSingletons: g.inlineSingletons}
	sg.indyLambda(x)
	s.clinit.FieldInsn(cf.PUTSTATIC, s.cls.Name, name, fd)

	getter := "get" + strings.ToUpper(name[:1]) + name[1:] + "$" + g.shared.opts.Module
	getters := []string{getter}
	if g.inlineSingletons {
		s.compat++
		getters = append(getters, "getLambda-"+strconv.Itoa(s.compat)+"$"+g.shared.opts.Module)
	}
	for _, m := range getters {
		code := s.cls.NewMethod(cf.AccPublic|cf.AccFinal, m, "()"+fd)
		code.AllocLocal("L" + s.cls.Name + ";")
		code.FieldInsn(cf.GETSTATIC, s.cls.Name, name, fd)
		code.Op(cf.ARETURN)
		if err := code.Err(); err != nil {
			panic(err)
		}
	}
	s.getters[x] = getter
	return getter
}

// composeSingletons returns the file's ComposableSingletons class, which
// it starts the first time: ComposableSingletons$FileKt, in the file's
// package, an object.
func (g *gen) composeSingletons() *composeSingletons {
	if s := g.shared.singletons; s != nil {
		return s
	}
	name := "ComposableSingletons$" + syntax.FacadeName(g.file.Path)
	if p := g.file.Package; p != "" {
		name = strings.ReplaceAll(p, ".", "/") + "/" + name
	}
	c := cf.NewClass(cf.AccPublic|cf.AccFinal|cf.AccSuper, name, "java/lang/Object")
	c.SourceFile = filepath.Base(g.file.Path)
	c.AddField(cf.AccPublic|cf.AccStatic|cf.AccFinal, "INSTANCE", "L"+name+";")
	init := c.NewMethod(cf.AccPublic, "<init>", "()V")
	init.Local(cf.ALOAD, init.AllocLocal("L"+name+";"))
	init.Invoke(cf.INVOKESPECIAL, "java/lang/Object", "<init>", "()V")
	init.Op(cf.RETURN)
	if err := init.Err(); err != nil {
		panic(err)
	}
	clinit := c.NewMethod(cf.AccStatic, "<clinit>", "()V")
	clinit.New(name)
	clinit.Op(cf.DUP)
	clinit.Invoke(cf.INVOKESPECIAL, name, "<init>", "()V")
	clinit.FieldInsn(cf.PUTSTATIC, name, "INSTANCE", "L"+name+";")
	g.shared.singletons = &composeSingletons{cls: c, clinit: clinit, getters: map[*syntax.Lambda]string{}, names: map[string]bool{}}
	return g.shared.singletons
}

// finishSingletons completes the file's ComposableSingletons class, if it
// has one, and returns out with it.
func (g *gen) finishSingletons(out []Output) []Output {
	s := g.shared.singletons
	if s == nil {
		return out
	}
	s.clinit.Op(cf.RETURN)
	if err := s.clinit.Err(); err != nil {
		panic(err)
	}
	g.annotateMeta(s.cls, kmeta.WriteSynthetic())
	g.shared.classes[s.cls.Name] = s.cls
	return append(out, Output{Name: s.cls.Name})
}

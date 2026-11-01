// Package check resolves names and types in parsed files and annotates
// the AST for code generation.
package check

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/barteo/kotlin-compiler-go/internal/classfile"
	"github.com/barteo/kotlin-compiler-go/internal/classpath"
	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// Universe holds every top-level symbol of the compilation, by package.
// It is built sequentially and is read-only during the parallel
// body-checking phase, except for Java classes, which are loaded on demand.
type Universe struct {
	Pkgs     map[string]*Package // by dotted name; "" is the default package
	prefixes map[string]bool     // the packages and their prefixes: a and a.b for a.b
	ext      *classTable
	// memberExts is set if a class of the program declares extension
	// functions, which calls with lambdas' implicit receivers then look
	// for in their classes.
	memberExts bool
	// builtinNames is set if the program declares a class named as a
	// built-in type, as TextValue.String, which hides it where it is
	// in scope.
	builtinNames bool
	// noJVMDefault is Library.NoJVMDefault, and jvmDefaultCompat
	// Library.JVMDefaultCompat.
	noJVMDefault, jvmDefaultCompat bool
	// parcelize is Library.Parcelize, and serialization
	// Library.Serialization, and compose Library.Compose.
	parcelize, serialization, compose bool
	// parcelSealed is parcelizeSealed's, with parcelize.
	parcelSealed map[string]bool
	// serialInterfaces are the sealed interfaces annotated @Serializable,
	// with serialization.
	serialInterfaces map[*syntax.ClassDecl]bool
	// module ends the JVM names of internal members: $ and the module's
	// name.
	module string
	// aliasFile is the checker of the file that declares each typealias,
	// which resolves its right side in the file's scope.
	aliasFile map[*types.AliasSym]*Checker
}

// Package holds the top-level declarations of one package of the program.
type Package struct {
	Name    string
	Classes map[string]*types.ClassSym
	Funs    map[string][]*types.FunSym
	Vars    map[string]*types.VarSym
	// PrivateVars are the files' private top-level properties, which
	// other files only see to report that they can't.
	PrivateVars map[string]*types.VarSym
	// ExtProps are the getters of the package's extension properties, by
	// the properties' names.
	ExtProps map[string][]*types.FunSym
	Aliases  map[string]*types.AliasSym
}

// pkg returns a package of the program, creating it on first use.
func (u *Universe) pkg(name string) *Package {
	p := u.Pkgs[name]
	if p == nil {
		p = &Package{Name: name, Classes: map[string]*types.ClassSym{}, Funs: map[string][]*types.FunSym{}, Vars: map[string]*types.VarSym{},
			PrivateVars: map[string]*types.VarSym{}, ExtProps: map[string][]*types.FunSym{}, Aliases: map[string]*types.AliasSym{}}
		u.Pkgs[name] = p
		for n := name; n != ""; {
			u.prefixes[n] = true
			i := strings.LastIndexByte(n, '.')
			if i < 0 {
				break
			}
			n = n[:i]
		}
	}
	return p
}

// isPackage reports whether a dotted name is a package of the program, or
// a prefix of one: a in a.b.C.
func (u *Universe) isPackage(name string) bool { return u.prefixes[name] }

var builtins = map[string]*types.FunSym{
	"println": {Name: "println", Ret: types.UnitType, Builtin: types.Println, Resolved: true},
	"print":   {Name: "print", Ret: types.UnitType, Builtin: types.Print, Resolved: true},
}

type funKey struct {
	name string
	ext  bool
}

type loopCtx struct {
	node     syntax.Stmt // the *syntax.While or *syntax.For
	label    string
	hasBreak bool
	lambdas  int // len(c.lambdas) where the loop is
}

// Checker checks one file. Only one goroutine uses a Checker at a time.
type Checker struct {
	u       *Universe
	file    *syntax.File
	pkg     *Package                   // the file's package
	imports map[string]*types.ClassSym // explicitly imported classes by name
	// aliasImports are the program's explicitly imported typealiases.
	aliasImports map[string]*types.AliasSym
	funImports   map[string][]*types.FunSym // explicitly imported Kotlin functions of the class path by name
	// memberImports are the explicitly imported members of classes and
	// objects by name: enum entries, Java statics, objects' members, and
	// those of the classes of starOwners.
	memberImports map[string]memberImport
	stars         []string // star-imported packages of the class path (internal names)
	// starOwners are the classes whose members and nested classes are
	// star-imported: import E.*.
	starOwners []*types.ClassSym
	// Imports of the program's own declarations.
	userFuns map[string][]*types.FunSym
	userVars map[string]*types.VarSym
	// fileVars are the file's private top-level properties, which other
	// files' private properties of the same name don't conflict with.
	fileVars     map[string]*types.VarSym
	userExtProps map[string][]*types.FunSym // getters of extension properties
	userStars    []*Package
	cyclic       []*syntax.ClassDecl     // classes that inherit from themselves
	classes      []*syntax.ClassDecl     // the file's classes, nested ones too
	aliases      []*syntax.TypeAliasDecl // the file's typealiases, nested ones too
	temps        int                     // temporaries named so far, for unique names
	inlineCalls  []inlineCall            // calls among the program's inline functions, for cycles
	inlineDecls  []*syntax.FunDecl       // the file's inline functions that are copied into callers
	// lookupCls is the class whose signatures are being resolved, for its
	// nested classes' names; in bodies, cls is.
	lookupCls *types.ClassSym
	// supersOf is the class whose supertypes are being resolved, which
	// may name its private nested classes: class A : I<A.N>.
	supersOf *types.ClassSym
	// anon are the classes of the file's object expressions, with the
	// code around each; anonCount numbers them, by enclosing function.
	anon      map[*types.ClassSym]*anonCtx
	anonCount map[string]int
	// deferred counts the locals declared without a value, and
	// localDelegates is set if the file has local delegated properties:
	// without them, the passes that look for them are skipped.
	deferred       int
	localDelegates bool
	// Results of name lookups that search the class path, which are the
	// same throughout the file. They spare the shared tables' locks.
	classCache map[string]classifier
	funCache   map[funKey][]*types.FunSym
	levelCache map[funKey][][]*types.FunSym // see kotlinFunLevels
	errs       []*syntax.Error
	// shadowed are the calls whose implicit receiver a context parameter
	// shadows, reported once checked (see receiverShadowed).
	shadowed map[syntax.Expr]shadowedUse
	// dslUses are the uses of implicit values that DSL markers don't
	// allow, reported once checked, and markers the markers of classes
	// (see dsl.go).
	dslUses map[dslKey]shadowedUse
	markers map[*types.ClassSym][]*types.ClassSym
	// accessed names the properties with context parameters whose
	// accessors' calls reading or assigning them are (see contextAccess).
	accessed map[*syntax.Call]string
	done     map[any]bool // declarations already checked during inference
	// jumps says, for each inlined lambda, whether a non-local return,
	// break or continue can leave it.
	jumps map[*syntax.Lambda]inlineJumps
	// checked are lambda arguments checked while choosing an overload by
	// their result type, with the type of their bodies. postponedLambda
	// doesn't check them again.
	checked map[*syntax.Lambda]types.Type
	// refCount numbers the locals holding bound references' receivers.
	refCount int
	// stubs are the variables of builder lambdas' parameter types that
	// stubInference is inferring, with the run that records constraints on
	// them.
	stubs map[*types.TypeParamSym]*stubRun
	// trial is set while stubInference checks a lambda, to undo what the
	// check changes outside the lambda.
	trial *trial
	// redeclared are the classes that finished trials declared, which
	// declareClass declares again.
	redeclared map[*types.ClassSym]bool
	sink       *system // set while checking an argument whose inference joins its call's
	// noBoundChecks is set while class bounds are resolved, before
	// all of them are known.
	noBoundChecks bool
	// localClasses are the file's local classes' capture frames.
	localClasses map[*types.ClassSym]localClassFrame
	// tailBlock is a branch whose last statement, a call, is checked with
	// the inference system tailSink: see branch.
	tailBlock *syntax.Block
	tailSink  *system
	// bareRef is the type of an is or as being resolved, whose class's type
	// arguments may be left out, and bareSubject the type checked.
	bareRef     *syntax.TypeRef
	bareSubject types.Type
	// outerSubst binds the outer class's type parameters of an inner class
	// whose constructor call is being checked: see innerConstruct.
	outerSubst types.Subst
	// noCapture is an implicit receiver that a lookup tries, which this
	// mustn't capture unless the lookup finds the name: see recvMember.
	noCapture *types.VarSym
	// Smart casts of local vars (see flow.go): where each is declared,
	// whether code outside a frame reading it assigns it, the types that
	// nested code assigns it, the next block's exit to record, and the
	// statement being checked.
	varSites      map[*types.VarSym]varSite
	concurrent    map[varFrame]bool
	nestedAssigns map[*types.VarSym][]types.Type
	assignsMemo   map[any][]string
	// implied has the smart casts that a local val's not being null
	// implies: those of the safe calls of its value, as K2's for
	// val t = a?.b, where t != null means a != null.
	implied map[*types.VarSym][]narrow
	// conds has the smart casts that a local Boolean val's being true
	// and false imply, those of its value, as K2's for
	// val ok = x != null; if (ok) x.length.
	conds map[*types.VarSym][2][]narrow
	// paths has the values of the stable properties of stable values,
	// a.b of a.b.c, that smart casts are of.
	paths map[[2]*types.VarSym]*types.VarSym
	// blockScope is the scope of the block checked last, after it.
	blockScope *scope
	exitWatch  *branchExit
	site       varSite
	state      // per-declaration state, saved around nested inference
	// serials are the file's classes annotated @Serializable, which the
	// serialization plugin adds members to: see serialization.go.
	serials []*serialInfo
}

type state struct {
	scope      *scope
	cls        *types.ClassSym
	this       *types.VarSym // what smart casts of this narrow; nil outside classes
	fn         *types.FunSym // nil in property initializers and init blocks
	retType    types.Type
	retAllowed bool
	loops      []*loopCtx
	allowRange bool
	tparams    []*types.TypeParamSym // type parameters in scope, innermost first
	lambdas    []*syntax.Lambda      // lambdas being checked, outermost first
	recvs      []implicitRecv        // the receivers of lambdas with receivers, innermost last
	ctxs       []implicitRecv        // the context parameters in scope, innermost last: see context.go
	lrets      []*lambdaResult       // the labeled returns of each of lambdas
	// defaultsOf is the function whose default value of parameter
	// defaultParam is being checked.
	defaultsOf   *types.FunSym
	defaultParam int
	// staticCls is set where a class's companion members are in scope
	// but this isn't: in the arguments of its constructors' this(...) and
	// super(...) calls.
	staticCls *types.ClassSym
	// ctorArgs is set while the arguments of an enum entry's or an
	// annotation's constructor call are checked: on the JVM, the object
	// is already on the stack, uninitialized. It isn't set for those of
	// this(...) and super(...), whose receiver, this, can be kept in a
	// local meanwhile, nor for other constructor calls, whose arguments
	// codegen evaluates first if they have a try.
	ctorArgs bool
	// base is the number of lambdas outside the function being checked:
	// in a local function, its frame and those around it, which return,
	// break and continue can't leave.
	base int
	// localCalls are the calls of local functions in the body being
	// checked, shared by the local functions and lambdas in it.
	localCalls *[]localCall
	// loopsOutside is set in a local function declared in a loop, which
	// break and continue can't leave.
	loopsOutside bool
	// init is set in a class's constructor code: property initializers,
	// init blocks and secondary constructors, which can assign its vals
	// declared without a value.
	init bool
	// thisBase is the number of lambdas that don't capture this: in an
	// object expression's members, the ones around the object, whose
	// this is another.
	thisBase int
	// lambdaClass is set in a suspend lambda's body: the name of its
	// class, which the classes in it are named after.
	lambdaClass string
	// fnName is set in a local function: its name, after the names of
	// the functions it is in, which name the classes in it, as kotlinc
	// names them: outer$local.
	fnName string
	// declName is the variable or property whose value is being checked,
	// which names the classes of object expressions in it.
	declName string
}

// noWant means there is no expected type.
var noWant = types.InvalidType

// Check type-checks all files, using up to jobs goroutines for function
// bodies. Java classes are looked up in cp, which may be nil. It returns
// all diagnostics sorted by file and position.
func Check(files []*syntax.File, cp *classpath.Path, jobs int) []*syntax.Error {
	return CheckWith(files, NewLibrary(cp), jobs)
}

// Library is what the checker reads from the class path: Java classes and
// Kotlin declarations. It is safe for concurrent use.
type Library struct {
	t *classTable
	// Module is the name of the module being compiled, which the JVM
	// names of its internal members end with, as with kotlinc; "main" if
	// empty.
	Module string
	// NoJVMDefault is set to compile the program's interfaces without JVM
	// default methods, as kotlinc's -jvm-default=disable does: see
	// types.ClassSym.HasDefaultImpls.
	NoJVMDefault bool
	// JVMDefaultCompat is set to compile the program's interfaces with
	// their bodies also in DefaultImpls classes, as kotlinc's
	// -jvm-default=enable does: see types.ClassSym.JVMCompat.
	JVMDefaultCompat bool
	// Parcelize is set to compile as kotlinc does with the parcelize
	// compiler plugin: see parcelize.go.
	Parcelize bool
	// Serialization is set, by EnableSerialization, to compile as kotlinc
	// does with the kotlinx.serialization compiler plugin: see
	// serialization.go.
	Serialization bool
	// Compose is set to compile as kotlinc does with the Compose compiler
	// plugin: see compose.go.
	Compose bool
}

// EnableSerialization sets Serialization. The code that kc writes for the
// plugin uses the internal declarations of kotlinx.serialization's
// runtime, as the plugin's does, so the classes of its package
// kotlinx.serialization.internal, and UnknownFieldException, are then read
// as a friend path's. It is called before the library is read.
func (l *Library) EnableSerialization() {
	l.Serialization = true
	l.t.serialization = true
}

// NewLibrary returns the library of the classes in cp, which may be nil.
func NewLibrary(cp *classpath.Path) *Library { return &Library{t: newClassTable(cp)} }

// moduleSuffix returns what the JVM names of a module's internal members
// end with: $ and its name, with the characters that can't be in a Java
// identifier replaced by _, as kotlinc writes them.
func moduleSuffix(module string) string {
	if module == "" {
		module = "main"
	}
	b := []byte(module)
	for i, r := range b {
		if !(r == '_' || r == '$' || '0' <= r && r <= '9' || 'a' <= r && r <= 'z' || 'A' <= r && r <= 'Z') {
			b[i] = '_'
		}
	}
	return "$" + string(b)
}

// internal returns the suffix of the JVM name of a member of a class
// declared with modifiers m: the module's, if it is internal, as kotlinc
// mangles them, but for those that are @PublishedApi, which inline
// functions of other modules call, or renamed by @JvmName.
func (c *Checker) internal(m *syntax.Modifiers, owner *types.ClassSym, target string) string {
	if owner == nil || m.Visibility != syntax.Internal || m.Find("", "PublishedApi") != nil || jvmName(m, target) != "" {
		return ""
	}
	return c.u.module
}

// Prefetch reads the Kotlin standard library's default-imported packages,
// which almost every program uses, so checking finds them read. It does
// nothing without the standard library. It can run while the sources are
// parsed.
func (l *Library) Prefetch() {
	t := l.t
	if !t.kotlin {
		return
	}
	var wg sync.WaitGroup
	wg.Go(func() { t.builtinClasses() })
	for _, pkg := range defaultPackages {
		wg.Go(func() { t.pkg(pkg) })
	}
	wg.Wait()
}

// CheckWith is Check with a library that may have been prefetched.
func CheckWith(files []*syntax.File, lib *Library, jobs int) []*syntax.Error {
	u := &Universe{Pkgs: map[string]*Package{}, prefixes: map[string]bool{}, ext: lib.t, module: moduleSuffix(lib.Module),
		aliasFile: map[*types.AliasSym]*Checker{}, noJVMDefault: lib.NoJVMDefault, jvmDefaultCompat: lib.JVMDefaultCompat, parcelize: lib.Parcelize, serialization: lib.Serialization,
		compose: lib.Compose, serialInterfaces: map[*syntax.ClassDecl]bool{}}
	checkers := make([]*Checker, len(files))
	for i, f := range files {
		checkers[i] = &Checker{u: u, file: f, done: map[any]bool{}, checked: map[*syntax.Lambda]types.Type{},
			jumps:    map[*syntax.Lambda]inlineJumps{},
			varSites: map[*types.VarSym]varSite{}, concurrent: map[varFrame]bool{}, nestedAssigns: map[*types.VarSym][]types.Type{}, assignsMemo: map[any][]string{},
			classCache: map[string]classifier{}, funCache: map[funKey][]*types.FunSym{},
			levelCache: map[funKey][][]*types.FunSym{}}
	}
	inheritDefaults(files)
	dropContracts(files)
	if u.parcelize {
		u.parcelSealed = parcelizeSealed(checkers)
	}
	for _, c := range checkers {
		c.declare()
	}
	if lib.t.cp != nil && lib.t.cp.HasSources() {
		find := u.sourceClass
		lib.t.sources.Store(&find)
	}
	expectActual(checkers)
	// Imports can name declarations of any file.
	for _, c := range checkers {
		c.resolveImports()
	}
	// Typealiases can stand for supertypes.
	for _, c := range checkers {
		c.resolveAliases()
	}
	for _, c := range checkers {
		c.resolveSupertypes()
	}
	for _, c := range checkers {
		c.resolveClassBounds()
	}
	for _, c := range checkers {
		c.findCycles()
	}
	for _, c := range checkers {
		c.checkSupers()
	}
	for _, c := range checkers {
		c.resolveSignatures()
	}
	forwardInlinable(checkers)
	for _, c := range checkers {
		c.checkMemberImports()
	}
	for _, c := range checkers {
		c.inheritDefaults()
	}
	for _, c := range checkers {
		c.delegations()
	}
	for _, c := range checkers {
		c.defaultImpls()
	}
	// Omitted return and property types are inferred up front, in order, so
	// the parallel phase only reads signatures.
	for _, c := range checkers {
		c.inferAll()
	}
	for _, c := range checkers {
		c.serialBodies()
	}
	for _, c := range checkers {
		c.boxedReturns()
	}
	for _, c := range checkers {
		c.checkClashes()
	}

	if jobs < 1 {
		jobs = 1
	}
	var wg sync.WaitGroup
	work := make(chan *Checker)
	for range jobs {
		wg.Go(func() {
			for c := range work {
				c.checkBodies()
			}
		})
	}
	for _, c := range checkers {
		work <- c
	}
	close(work)
	wg.Wait()

	var errs []*syntax.Error
	for _, c := range checkers {
		c.shadowedErrors()
		c.dslErrors()
		sort.SliceStable(c.errs, func(i, j int) bool {
			a, b := c.errs[i].Pos, c.errs[j].Pos
			return a.Line < b.Line || a.Line == b.Line && a.Col < b.Col
		})
		errs = append(errs, c.errs...)
	}
	return errs
}

// sourceClass returns the program's class with an internal name, as
// a/b/Outer$Inner, or nil.
func (u *Universe) sourceClass(name string) *types.ClassSym {
	pkg, rest := "", name
	if i := strings.LastIndexByte(name, '/'); i >= 0 {
		pkg, rest = strings.ReplaceAll(name[:i], "/", "."), name[i+1:]
	}
	p := u.Pkgs[pkg]
	if p == nil {
		return nil
	}
	parts := strings.Split(rest, "$")
	cls := p.Classes[parts[0]]
	for _, n := range parts[1:] {
		if cls == nil {
			return nil
		}
		cls = cls.Nested[n]
	}
	return cls
}

func (c *Checker) errorf(p syntax.Pos, format string, args ...any) {
	c.errs = append(c.errs, &syntax.Error{File: c.file.Path, Pos: p, Msg: fmt.Sprintf(format, args...)})
}

// unsupported reports the use of a Kotlin feature kc doesn't support,
// with a stable name for it, as the parser does.
func (c *Checker) unsupported(p syntax.Pos, feature, format string, args ...any) {
	c.errs = append(c.errs, &syntax.Error{File: c.file.Path, Pos: p, Msg: fmt.Sprintf(format, args...), Feature: feature})
}

// unsupportedTypes names the feature of built-in Kotlin types kc doesn't
// support.
var unsupportedTypes = map[string]string{
	"UInt": "unsigned integers", "ULong": "unsigned integers", "UByte": "unsigned integers", "UShort": "unsigned integers",
	"UIntArray": "unsigned integers", "ULongArray": "unsigned integers", "UByteArray": "unsigned integers", "UShortArray": "unsigned integers",
}

// hidesBuiltin reports whether a class of the program that is in scope,
// local, nested in an enclosing class (or its supertypes), imported or of
// the file's package, hides the built-in type of a name.
func (c *Checker) hidesBuiltin(name string) bool {
	if c.scope != nil && c.scope.findClass(name) != nil {
		return true
	}
	if k := c.nestedClassifier(name); k.cls != nil && !k.cls.External {
		return true
	}
	if cls, ok := c.imports[name]; ok && !cls.External {
		return true
	}
	_, ok := c.pkg.Classes[name]
	return ok
}

func (c *Checker) resolveType(ref *syntax.TypeRef) types.Type {
	if ref.Known != nil {
		return *ref.Known
	}
	if ref.And != nil {
		return c.defNotNull(ref)
	}
	if ref.Ret != nil {
		return c.resolveFuncType(ref)
	}
	if ref.Outer != nil {
		return c.innerType(ref)
	}
	if a := c.lookupAlias(ref.Name); a != nil && c.lookupTypeParam(ref.Name) == nil {
		return c.expandAlias(a, ref)
	}
	var t types.Type
	if tp := c.lookupTypeParam(ref.Name); tp != nil {
		if ref.Args != nil {
			c.errorf(ref.Args[0].P, "type arguments are not allowed for type parameters")
			return types.InvalidType
		}
		t = types.ParamType(tp)
	} else {
		name := ref.Name
		if b, ok := strings.CutPrefix(name, "kotlin."); ok && kotlinTypes[b] {
			name = b // the built-in type, also where a class of the program hides it
		} else if c.u.builtinNames && kotlinTypes[name] && c.hidesBuiltin(name) {
			name = "" // the class
		}
		switch name {
		case "Int":
			t = types.IntType
		case "Long":
			t = types.LongType
		case "Double":
			t = types.DoubleType
		case "Boolean":
			t = types.BoolType
		case "Char":
			t = types.CharType
		case "Float":
			t = types.FloatType
		case "Byte":
			t = types.ByteType
		case "Short":
			t = types.ShortType
		case "String":
			t = types.StringType
		case "Unit":
			t = types.UnitType
		case "Nothing":
			t = types.NothingType
		case "Any":
			t = types.AnyType
		default:
			cls := c.lookupClass(ref.Name)
			if cls == nil {
				if f, ok := unsupportedTypes[ref.Name]; ok {
					c.unsupported(ref.P, f, "unresolved or unsupported type: %s", ref.Name)
				} else {
					c.errorf(ref.P, "unresolved or unsupported type: %s", ref.Name)
				}
				return types.InvalidType
			}
			if !c.usable(cls, ref.P) {
				return types.InvalidType
			}
			c.checkClassVisible(cls, ref.P)
			return c.applyTypeArgs(cls, ref)
		}
		if ref.Args != nil {
			c.errorf(ref.Args[0].P, "no type arguments expected for class %s", ref.Name)
			return types.InvalidType
		}
	}
	return c.nullable(t, ref)
}

// resolveFuncType resolves a function type (A, B) -> R.
// reservedRef keys Checker.done for a function type whose reserved
// context parameter is reported: types are resolved more than once.
type reservedRef struct{ *syntax.TypeRef }

func (c *Checker) resolveFuncType(ref *syntax.TypeRef) types.Type {
	if len(ref.Params) > types.MaxVarArity {
		c.errorf(ref.P, "function types with more than %d parameters are not supported", types.MaxVarArity)
		return types.InvalidType
	}
	ps := make([]types.Type, len(ref.Params))
	for i, p := range ref.Params {
		if ps[i] = c.resolveType(p); ps[i].K == types.Invalid {
			return types.InvalidType
		}
	}
	r := c.resolveType(ref.Ret)
	if r.K == types.Invalid {
		return types.InvalidType
	}
	if ref.Recv != nil {
		// R.(A) -> T is (R, A) -> T, marked as having a receiver.
		recv := c.resolveType(ref.Recv)
		if recv.K == types.Invalid {
			return types.InvalidType
		}
		ps = append([]types.Type{recv}, ps...)
	}
	if len(ref.Contexts) > 0 {
		// context(C) R.(A) -> T is (C, R, A) -> T, marked as having a
		// context parameter.
		ctx := make([]types.Type, len(ref.Contexts))
		for i, p := range ref.Contexts {
			if ctx[i] = c.resolveType(p); ctx[i].K == types.Invalid {
				return types.InvalidType
			}
			if reservedContext(ctx[i]) && !c.done[reservedRef{ref}] {
				c.done[reservedRef{ref}] = true
				c.reservedContextError(ref.P)
			}
		}
		ps = append(ctx, ps...)
	}
	var t types.Type
	switch {
	case ref.Composable && c.u.compose && (ref.Suspend || len(ps) > types.MaxComposableArity):
		c.unsupported(ref.P, "compose", "composable function types that suspend or have more than %d parameters are not supported", types.MaxComposableArity)
		return types.InvalidType
	case ref.Composable && c.u.compose:
		t = c.u.ext.funcs.ComposableType(ps, r)
	case ref.Suspend && !c.coroutines(ref.P):
		return types.InvalidType
	case ref.Suspend && len(ps)+1 > types.MaxVarArity:
		c.errorf(ref.P, "suspend function types with more than %d parameters are not supported", types.MaxVarArity-1)
		return types.InvalidType
	case ref.Suspend:
		t = c.u.ext.funcs.SuspendType(ps, r)
	case !c.arityOK(ref.P, len(ps), "function types"):
		return types.InvalidType
	default:
		c.useArity(len(ps))
		t = c.u.ext.funcs.Type(ps, r)
	}
	if ref.Recv != nil {
		t = t.WithReceiver()
	}
	if len(ref.Contexts) > 0 {
		t = t.WithContexts(len(ref.Contexts))
	}
	return c.nullable(t, ref)
}

// coroutines reports whether suspend functions can be compiled: they
// need the standard library's continuations. If not, it reports an error
// at p.
func (c *Checker) coroutines(p syntax.Pos) bool {
	if !c.u.ext.kotlin {
		c.errorf(p, "suspend functions need the Kotlin standard library")
		return false
	}
	return true
}

func (c *Checker) nullable(t types.Type, ref *syntax.TypeRef) types.Type {
	if ref.Nullable {
		if t.K == types.Unit {
			// Unit?, the type of kotlin.Unit or null: Unit itself has no
			// value on the JVM, so this is the object's class.
			u := c.u.ext.class("kotlin/Unit")
			if u == nil {
				c.errorf(ref.P, "Unit? is not supported")
				return types.InvalidType
			}
			return types.ClassType(u).Nullable()
		}
		t = t.Nullable()
	}
	return t
}

// applyTypeArgs resolves a reference to a class with its type arguments,
// which must match the class's type parameters and their bounds.
func (c *Checker) applyTypeArgs(cls *types.ClassSym, ref *syntax.TypeRef) types.Type {
	if cls.Inner && cls.OuterParams > 0 {
		return c.innerType(ref) // Inner in its outer class's code
	}
	ps := cls.TypeParams
	if ref.Args == nil && len(ps) > 0 && ref == c.bareRef {
		return c.bareType(cls, ref, c.bareSubject)
	}
	if len(ref.Args) != len(ps) {
		p := ref.P
		if ref.Args != nil {
			p = ref.Args[0].P
		}
		if len(ps) == 0 {
			c.errorf(p, "no type arguments expected for class %s", cls.DisplayName())
		} else {
			c.errorf(p, "%s expected for class %s%s", countArgs(len(ps)), cls.DisplayName(), types.TypeParamsString(ps))
		}
		return types.InvalidType
	}
	if len(ps) == 0 {
		return c.nullable(types.ClassType(cls), ref)
	}
	args := make([]types.Arg, len(ps))
	for i, a := range ref.Args {
		args[i] = types.Arg{V: a.Variance}
		if a.Variance == types.Star {
			continue
		}
		args[i].T = c.resolveType(a.Type)
		if args[i].T.K == types.Invalid {
			return types.InvalidType
		}
		if a.Variance != types.Inv && ps[i].Var != types.Inv && a.Variance != ps[i].Var {
			c.errorf(a.P, "projection is conflicting with variance of the corresponding type parameter of %s", cls.DisplayName())
		}
	}
	t := types.GenericType(cls, args)
	if !c.noBoundChecks {
		c.checkBounds(t, ref)
	}
	return c.nullable(t, ref)
}

// checkBounds checks the type arguments of t, written as ref, against the
// bounds of its class's type parameters. A typealias's arguments needn't
// be its expansion's, as in One<Any> for TK<Any, Any>: errors in those
// are reported on the whole reference.
func (c *Checker) checkBounds(t types.Type, ref *syntax.TypeRef) {
	ps := t.C.TypeParams
	s := classSubst(t)
	for i, a := range t.A.List {
		if a.V == types.Star {
			continue
		}
		p := ref.P
		if len(ref.Args) == len(t.A.List) {
			p = ref.Args[i].P
		}
		for _, b := range ps[i].Bounds {
			if b = s.Apply(b); !c.subtype(a.T, b) {
				c.errorf(p, "type argument is not within its bounds: type parameter '%s' must be subtype of '%s', but actual: '%s'", ps[i].Name, b, a.T)
			}
		}
	}
}

// lookupTypeParam finds a type parameter in scope by name.
func (c *Checker) lookupTypeParam(name string) *types.TypeParamSym {
	for _, p := range c.tparams {
		if p.Name == name {
			return p
		}
	}
	// An object expression's signatures see the enclosing code's, and so
	// do its inner classes'.
	for k := c.lookupCls; k != nil && (k.Anonymous || k.Inner); k = k.Outer {
		for _, p := range c.anon[k].tparams {
			if p.Name == name {
				return p
			}
		}
	}
	return nil
}

// declareTypeParams creates the symbols for declared type parameters.
func (c *Checker) declareTypeParams(decls []*syntax.TypeParam, owner any, allowVariance bool) []*types.TypeParamSym {
	var out []*types.TypeParamSym
	for i, d := range decls {
		for _, e := range decls[:i] {
			if e.Name == d.Name {
				c.errorf(d.P, "conflicting declarations: %s", d.Name)
			}
		}
		if d.Variance != types.Inv && !allowVariance {
			c.errorf(d.P, "variance annotations are only allowed for type parameters of classes")
		}
		d.Sym = &types.TypeParamSym{Name: d.Name, Var: d.Variance, Index: i, Owner: owner, Reified: d.Reified}
		if d.Reified {
			if _, fn := owner.(*types.FunSym); !fn {
				c.errorf(d.P, "only type parameters of inline functions can be reified")
			}
		}
		out = append(out, d.Sym)
	}
	return out
}

// resolveBounds resolves the bounds of type parameters, with the
// parameters themselves in scope: T : Comparable<T>.
func (c *Checker) resolveBounds(decls []*syntax.TypeParam) {
	// A bound is checked against its own parameters' bounds once they
	// are all set: T : Enum<T> needs T's bound to check Enum<T>.
	saved := c.noBoundChecks
	c.noBoundChecks = true
	refs := map[*syntax.TypeParam][]*syntax.TypeRef{} // the TypeRef of each bound in Bounds
	for _, d := range decls {
		d.Sym.Bounds = nil
		for _, ref := range d.Bounds {
			if b := c.resolveType(ref); b.K != types.Invalid {
				d.Sym.Bounds = append(d.Sym.Bounds, b)
				refs[d] = append(refs[d], ref)
			}
		}
		c.checkBoundKinds(d, refs[d])
	}
	// A parameter can't be bounded by itself, T : T or E : E & Any, or
	// through others, T : U, U : T. Such a bound is dropped, so that T
	// has a bound to erase to.
	var cyclic []*syntax.TypeParam
	for _, d := range decls {
		seen := map[*types.TypeParamSym]bool{}
		for p := d.Sym; len(p.Bounds) == 1 && p.Bounds[0].IsParam() && !seen[p]; {
			seen[p] = true
			if p = p.Bounds[0].Param(); p == d.Sym {
				cyclic = append(cyclic, d)
				break
			}
		}
	}
	for _, d := range cyclic {
		c.errorf(d.P, "type parameter %s has cyclic upper bounds", d.Name)
		d.Sym.Bounds = nil
	}
	c.noBoundChecks = saved
	if saved {
		return
	}
	for _, d := range decls {
		if len(refs[d]) != len(d.Sym.Bounds) {
			continue // a cyclic bound was dropped
		}
		for i, b := range d.Sym.Bounds {
			if ref := refs[d][i]; b.K == types.Class && b.A != nil && len(ref.Args) == len(b.A.List) {
				c.checkBounds(b.NonNull(), ref)
			}
		}
	}
}

// checkBoundKinds reports what kotlinc reports of a type parameter's
// several bounds: only one of them can be a class, and a type parameter
// can't be one of them.
func (c *Checker) checkBoundKinds(d *syntax.TypeParam, refs []*syntax.TypeRef) {
	if len(d.Sym.Bounds) < 2 {
		return
	}
	classes := 0
	for i, b := range d.Sym.Bounds {
		switch {
		case b.IsParam():
			c.errorf(refs[i].P, "type parameter cannot have any other bounds if it's bounded by another type parameter")
		case b.K == types.Class && !b.C.Interface:
			if classes++; classes == 2 {
				c.errorf(refs[i].P, "only one of the upper bounds can be a class")
			}
		}
	}
}

// resolveClassBounds resolves the bounds of class type parameters, which
// other files' signatures may need. Bounds are checked against each
// other only where they are used.
func (c *Checker) resolveClassBounds() {
	c.noBoundChecks = true
	for _, d := range c.classes {
		c.tparams, c.lookupCls = d.Sym.TypeParams, d.Sym
		c.resolveBounds(d.TypeParams)
	}
	c.tparams, c.lookupCls = nil, nil
	c.noBoundChecks = false
}

// declare enters the file's top-level names into its package.
func (c *Checker) declare() {
	c.errs = append(c.errs, c.file.Errors...)
	c.dropExtensions()
	c.serializableWith()
	c.parcelize()
	c.serialization()
	c.pkg = c.u.pkg(c.file.Package)
	prefix := ""
	if c.file.Package != "" {
		prefix = strings.ReplaceAll(c.file.Package, ".", "/") + "/"
	}
	for _, d := range c.file.Decls {
		switch d := d.(type) {
		case *syntax.FunDecl:
			sym := &types.FunSym{Name: d.Name, JVMName: jvmName(&d.Modifiers, ""), Facade: c.declFacade(d.Visibility), Decl: d, Ext: d.Recv != nil, Hidden: d.Hidden(),
				Private: privateMethod(&d.Modifiers, nil), Module: c.partSuffix(d.Visibility)}
			d.Sym = sym
			c.pkg.Funs[d.Name] = append(c.pkg.Funs[d.Name], sym)
		case *syntax.PropDecl:
			if d.Recv != nil {
				c.declareExtProp(d)
				continue
			}
			private := d.Visibility == syntax.Private
			prev, dup := c.fileVars[d.Name]
			if !dup && !private {
				prev, dup = c.pkg.Vars[d.Name]
			}
			alt := dup && altProp(prev, d)
			if dup && !alt {
				c.errorf(d.P, "conflicting declarations: %s", d.Name)
			}
			sym := &types.VarSym{Name: d.Name, Kind: types.Global, Mutable: d.Mutable, Facade: c.file.Class(), Decl: d,
				Const: d.Const, JvmField: d.Find("", "JvmField") != nil,
				JVMGetter: jvmName(&d.Modifiers, "get"), JVMSetter: jvmName(&d.Modifiers, "set")}
			if c.file.Part != "" && !private {
				// The facade's accessors call the part's, which use
				// its field.
				sym.Facade, sym.Part = c.file.Facade, c.file.Part
				switch {
				case sym.JvmField:
					c.errorf(d.Find("", "JvmField").P, "JvmField cannot be applied to top level property of a file annotated with JvmMultifileClass")
				case d.Lateinit:
					c.unsupported(d.P, "multifile class property", "lateinit properties of multi-file classes (@file:JvmMultifileClass) are only supported if private")
				}
			}
			sym.Private = d.Visibility == syntax.Private && d.Getter == nil && d.Setter == nil && d.Field == nil &&
				d.DelegateOf == nil && !sym.Const && !sym.JvmField
			d.Sym = sym
			if alt {
				addAlt(prev, sym)
			}
			if dup && prev.Alt != nil {
				continue // the pair stays
			}
			if private {
				if c.fileVars == nil {
					c.fileVars = map[string]*types.VarSym{}
				}
				c.fileVars[d.Name] = sym
				c.pkg.PrivateVars[d.Name] = sym
			} else {
				c.pkg.Vars[d.Name] = sym
			}
		case *syntax.TypeAliasDecl:
			if _, dup := c.pkg.Classes[d.Name]; dup {
				c.errorf(d.P, "redeclaration: typealias %s", d.Name)
			} else if _, dup := c.pkg.Aliases[d.Name]; dup {
				c.errorf(d.P, "redeclaration: typealias %s", d.Name)
			}
			c.pkg.Aliases[d.Name] = c.declareAlias(d, nil)
		case *syntax.ClassDecl:
			if _, dup := c.pkg.Classes[d.Name]; dup || c.pkg.Aliases[d.Name] != nil {
				c.errorf(d.P, "redeclaration: class %s", d.Name)
			}
			if d.Companion {
				c.errorf(d.P, "modifier 'companion' is only allowed on objects in classes")
			}
			c.pkg.Classes[d.Name] = c.declareClass(d, prefix+d.Name, nil)
			c.u.builtinNames = c.u.builtinNames || kotlinTypes[d.Name]
		}
	}
}

// declareClass creates the symbol of a class, an interface or an object,
// and of the classes nested in it. On the JVM, a nested class's name is
// its outer class's, $ and its own: Outer$Nested.
func (c *Checker) declareClass(d *syntax.ClassDecl, name string, outer *types.ClassSym) *types.ClassSym {
	c.funModifiers(d.Modifiers, d.P, "class")
	c.expectMembers(d)
	if d.Annotation {
		c.annotationClass(d)
	}
	if d.Value {
		c.valueClass(d)
	}
	sym := d.Sym
	if sym != nil && c.redeclared[sym] {
		// Declared by a trial check of the lambda the class is in, which
		// constraints on the lambda's types may mention: the class is
		// that one, declared again.
		delete(c.redeclared, sym)
	} else {
		sym = new(types.ClassSym)
	}
	*sym = types.ClassSym{Name: name, Methods: map[string][]*types.FunSym{}, Decl: d, Annotation: d.Annotation,
		Value: d.Value && !d.Interface && !d.Object, Fun: d.Fun,
		Interface: d.Interface || d.Annotation, Abstract: d.Abstract || d.Interface || d.Annotation || d.Sealed,
		Final:  d.Object || d.Enum || !d.Open && !d.Abstract && !d.Interface && !d.Sealed,
		Object: d.Object, Outer: outer, Nested: map[string]*types.ClassSym{}, Aliases: map[string]*types.AliasSym{}, Sealed: d.Sealed,
		File: c.file.Class()}
	if c.trial != nil {
		c.trial.classes[sym] = true
	}
	if d.Annotation {
		sym.Retention, sym.Targets = declaredAnnotation(&d.Modifiers)
	}
	sym.JVMDefault = d.Interface && !c.u.noJVMDefault
	if sym.JVMDefault {
		sym.JVMCompat = c.u.jvmDefaultCompat && d.Find("", "JvmDefaultWithoutCompatibility") == nil ||
			d.Find("", "JvmDefaultWithCompatibility") != nil
	}
	if sym.HasDefaultImpls() {
		c.noDefaultsChecks(d)
	}
	if d.Enum {
		sym.Entries = []string{}
		for _, e := range d.Entries {
			sym.Entries = append(sym.Entries, e.Name)
		}
		if d.Interface || d.Object || d.Data || d.Sealed || d.Open || d.Abstract {
			c.errorf(d.P, "modifier 'enum' is only allowed on final classes")
		}
		for _, m := range d.Members {
			// An enum may declare abstract members, which its entries
			// implement.
			switch m := m.(type) {
			case *syntax.FunDecl:
				sym.Abstract = sym.Abstract || m.Abstract
			case *syntax.PropDecl:
				sym.Abstract = sym.Abstract || m.Abstract
			}
		}
	}
	if d.Sealed && (d.Object || d.Open || d.Data) {
		c.errorf(d.P, "modifier 'sealed' is only allowed on classes and interfaces")
	}
	if d.Object && (d.Open || d.Abstract) {
		c.errorf(d.P, "modifiers open and abstract are not allowed on objects")
	}
	if d.Data && (d.Open || d.Abstract || d.Interface) {
		c.errorf(d.P, "modifier 'data' is incompatible with open, abstract and interfaces")
	}
	if d.Object && len(d.TypeParams) > 0 {
		c.errorf(d.P, "an object can't have type parameters")
	}
	olds := make([]*types.TypeParamSym, len(d.TypeParams))
	for i, tp := range d.TypeParams {
		if tp.Sym != nil && tp.Sym.Owner == sym {
			olds[i] = tp.Sym // the class is declared again: so are they
		}
	}
	sym.TypeParams = c.declareTypeParams(d.TypeParams, sym, true)
	for i, old := range olds {
		if old != nil {
			old.Redeclare(sym.TypeParams[i])
			sym.TypeParams[i], d.TypeParams[i].Sym = old, old
		}
	}
	if d.Inner {
		c.declareInner(d, sym, outer)
	}
	d.Sym = sym
	c.classes = append(c.classes, d)
	for _, m := range d.Members {
		if ad, ok := m.(*syntax.TypeAliasDecl); ok {
			if sym.Nested[ad.Name] != nil || sym.Aliases[ad.Name] != nil {
				c.errorf(ad.P, "redeclaration: typealias %s", ad.Name)
			}
			sym.Aliases[ad.Name] = c.declareAlias(ad, sym)
			continue
		}
		md, ok := m.(*syntax.ClassDecl)
		if !ok {
			continue
		}
		if md.EnumEntry {
			// An entry's class is a subclass of the enum, which its name
			// doesn't refer to.
			sym.Final = false
			c.declareClass(md, name+"$"+md.Name, sym)
			continue
		}
		if _, dup := sym.Nested[md.Name]; dup || sym.Aliases[md.Name] != nil {
			c.errorf(md.P, "redeclaration: class %s", md.Name)
		}
		n := c.declareClass(md, name+"$"+md.Name, sym)
		sym.Nested[md.Name] = n
		c.u.builtinNames = c.u.builtinNames || kotlinTypes[md.Name]
		if md.Companion {
			switch {
			case d.Object:
				c.errorf(md.P, "a companion object is not allowed in an object")
			case sym.Companion != nil:
				c.errorf(md.P, "only one companion object is allowed in a class")
			default:
				sym.Companion, n.CompanionOf = n, sym
			}
		}
	}
	return sym
}

// declFacade returns the class that calls of a top-level function or
// accessor with visibility vis go to: the file's facade, or for a file
// that is a part of a multi-file class, the class for one that isn't
// private, which calls the part's.
func (c *Checker) declFacade(vis syntax.Visibility) string {
	if c.file.Part != "" && vis == syntax.Private {
		return c.file.Part
	}
	return c.file.Facade
}

// partSuffix returns the end of the JVM name of a top-level function or
// accessor with visibility vis: for a private one of a multi-file class's
// part, $ and the part's simple name, as kotlinc names them, so that the
// parts' private functions don't clash.
func (c *Checker) partSuffix(vis syntax.Visibility) string {
	if c.file.Part == "" || vis != syntax.Private {
		return ""
	}
	return "$" + c.file.Part[strings.LastIndexByte(c.file.Part, '/')+1:]
}

// topFuns returns the program's top-level functions that a simple name
// can call, in Kotlin's order: explicit imports, the file's package, then
// star imports. An explicit import from the class path hides them.
func (c *Checker) topFuns(name string) []*types.FunSym {
	if levels := c.topFunLevels(name); len(levels) > 0 {
		return levels[0]
	}
	return nil
}

// topFunsFor is topFuns for call x: as in Kotlin, a level whose functions
// can't take the arguments' types gives way to the next one that can, as
// kotlinx.coroutines.time's delay(Duration) to kotlinx.coroutines'
// delay(Long) for delay(duration.coerceToMillis()).
func (c *Checker) topFunsFor(name string, x *syntax.Call) []*types.FunSym {
	levels := c.topFunLevels(name)
	switch len(levels) {
	case 0:
		return nil
	case 1:
		return levels[0]
	}
	ts := c.argTypes(x)
	for _, fs := range levels {
		if slices.ContainsFunc(fs, func(f *types.FunSym) bool { return c.mayTake(f, x) && c.takeTypes([]*types.FunSym{f}, x, ts) }) {
			return fs
		}
	}
	return levels[0]
}

// topFunLevels returns the levels of topFuns that have functions named
// name, in order.
func (c *Checker) topFunLevels(name string) [][]*types.FunSym {
	var levels [][]*types.FunSym
	if fs := nonExt(c.userFuns[name]); len(fs) > 0 {
		levels = append(levels, fs)
	}
	if _, ok := c.callableImport(name); ok || len(c.funImports[name]) > 0 {
		return levels
	}
	if fs := nonExt(c.pkg.Funs[name]); len(fs) > 0 {
		levels = append(levels, fs)
	}
	cls, ok := c.imports[name]
	if !ok {
		cls = c.pkg.Classes[name]
	}
	if cls != nil && !cls.Object && constructible(cls) {
		// A class imported explicitly or of the file's package comes
		// before the star imports: its constructor is called.
		return levels
	}
	var fs []*types.FunSym
	for _, p := range c.userStars {
		fs = append(fs, nonExt(p.Funs[name])...)
	}
	if len(fs) > 0 {
		levels = append(levels, fs)
	}
	return levels
}

// visible leaves out the private top-level functions of other files,
// which aren't candidates for call x, unless no other may take it: then
// the call reports that it can't access them.
func (c *Checker) visible(fs []*types.FunSym, x *syntax.Call) []*types.FunSym {
	if !slices.ContainsFunc(fs, c.hiddenFun) || !slices.ContainsFunc(fs, func(f *types.FunSym) bool { return !c.hiddenFun(f) && c.mayTake(f, x) }) {
		return fs
	}
	return slices.DeleteFunc(slices.Clone(fs), c.hiddenFun)
}

// hiddenFun reports whether the code can't see function f: another
// file's private top-level function, or a class's private or protected
// member extension.
func (c *Checker) hiddenFun(f *types.FunSym) bool {
	return !c.funVisible(f)
}

// topVar returns the program's top-level property that a simple name
// refers to, in the same order, or nil.
func (c *Checker) topVar(name string) *types.VarSym {
	return c.propAlt(c.topVar1(name))
}

func (c *Checker) topVar1(name string) *types.VarSym {
	if v, ok := c.userVars[name]; ok {
		return v
	}
	if c.valueImport(name) {
		return nil // an explicitly imported member comes first
	}
	if v, ok := c.fileVars[name]; ok {
		return v
	}
	if v, ok := c.pkg.Vars[name]; ok {
		return v
	}
	for _, p := range c.userStars {
		if v, ok := p.Vars[name]; ok {
			return v
		}
	}
	if c.u.ext.kotlin {
		// A Kotlin library's, by a star import, then the standard
		// library's default imports: DEFAULT_BUFFER_SIZE.
		for _, pkg := range c.stars {
			if v := c.u.ext.pkgVar(strings.ReplaceAll(pkg, "/", "."), name); v != nil {
				return v
			}
		}
		for _, pkg := range defaultPackages {
			if v := c.u.ext.pkgVar(pkg, name); v != nil {
				return v
			}
		}
	}
	if v, ok := c.pkg.PrivateVars[name]; ok {
		return v // another file's, which checkTopVar reports
	}
	return nil
}

// libraryVar reports whether v is a top-level property of a library, which
// a star import gives, rather than of the program.
func (c *Checker) libraryVar(v *types.VarSym) bool {
	_, ok := v.Decl.(*syntax.PropDecl)
	return !ok
}

func (c *Checker) resolveSignatures() {
	for _, d := range c.file.Decls {
		switch d := d.(type) {
		case *syntax.FunDecl:
			c.resolveFun(d, nil)
		case *syntax.PropDecl:
			c.resolveProp(d, nil)
		}
	}
	for _, d := range c.classes {
		c.lookupCls = d.Sym
		c.resolveClass(d)
	}
	c.lookupCls = nil
}

func (c *Checker) resolveFun(d *syntax.FunDecl, owner *types.ClassSym) {
	sym := d.Sym
	if sym == nil {
		sym = &types.FunSym{Name: d.Name, JVMName: jvmName(&d.Modifiers, ""), Owner: owner, Decl: d, Hidden: d.Hidden(),
			Module: c.internal(&d.Modifiers, owner, ""), Private: privateMethod(&d.Modifiers, owner)}
		d.Sym = sym
	}
	sym.Throws = c.throws(&d.Modifiers)
	if d.Annotation {
		what := "member function"
		if owner == nil {
			what = "top level function"
		}
		c.errorf(d.P, "modifier 'annotation' is not applicable to '%s'", what)
	}
	sym.TypeParams = c.declareTypeParams(d.TypeParams, sym, false)
	c.tparams = funTypeParams(sym, owner)
	defer func() { c.tparams = nil }()
	c.resolveBounds(d.TypeParams)
	if d.Recv != nil {
		c.declareRecv(d)
	}
	c.declareContexts(d, sym)
	c.memberModifiers(d.Modifiers, owner, d.P, "function '"+d.Name+"'")
	if d.Find("", "JvmStatic") != nil {
		switch {
		case owner == nil || !owner.Object || owner.Anonymous:
			c.errorf(d.P, "only members in named objects and companion objects can be annotated with '@JvmStatic'")
		case d.External && owner.CompanionOf != nil && owner.CompanionOf.Interface:
			c.errorf(d.P, "'@JvmStatic' annotation cannot be used on 'external' members of interface companions")
		case owner.CompanionOf != nil:
			// A static method of the outer class calls it: see staticStubs.
		case d.Recv != nil:
			c.unsupported(d.P, "@JvmStatic extension in object", "@JvmStatic is not supported on extension functions of objects")
		case d.Override:
			c.errorf(d.P, "override member cannot be '@JvmStatic' in an object")
		default:
			// Only a static method of the object's class, as kotlinc
			// writes it.
			sym.Static = true
		}
	}
	bodyless := d.Body == nil && d.ExprBody == nil
	sym.Abstract = d.Abstract || owner != nil && owner.Interface && bodyless
	sym.Final = owner == nil || isFinal(d.Modifiers, owner)
	sym.Operator, sym.Infix = d.Operator, d.Infix
	// Written bare under import kotlin.internal.*, as kotlinx.coroutines does.
	sym.LowPriority = d.Find("", "internal.LowPriorityInOverloadResolution") != nil || d.Find("", "LowPriorityInOverloadResolution") != nil
	sym.Suspend = d.Suspend && c.coroutines(d.P)
	switch {
	case d.External:
		sym.External = c.external(d, owner, d.Abstract, d.P) && bodyless
	case sym.Abstract && !bodyless:
		c.errorf(d.P, "an abstract function cannot have a body")
	case !sym.Abstract && bodyless:
		c.errorf(d.P, "function '%s' must have a body", d.Name)
	}
	seen := map[string]bool{}
	for _, p := range d.Params {
		if seen[p.Name] {
			c.errorf(p.P, "conflicting declarations: %s", p.Name)
		}
		seen[p.Name] = true
		p.Sym = c.paramSym(p, c.resolveType(p.Type), p)
		sym.Params = append(sym.Params, p.Sym)
	}
	c.checkVarargs(d.Params)
	c.composeFun(d, owner)
	if d.Once != nil {
		sym.Once = make([]bool, len(d.Params))
		for i, p := range d.Params {
			sym.Once[i] = d.Once[p.Name]
		}
	}
	c.contractEffects(d, sym)
	if d.Infix {
		c.checkInfix(d, owner)
	}
	switch {
	case d.Ret != nil:
		sym.Ret, sym.Resolved = c.resolveType(d.Ret), true
	case d.ExprBody == nil:
		sym.Ret, sym.Resolved = types.UnitType, true
	default:
		sym.Infer = func() { c.inferFun(d) }
	}
	if d.Operator {
		c.checkOperatorDecl(d, owner)
	}
	if d.Inline {
		c.inlineFun(d, owner)
	}
	for _, tp := range d.TypeParams {
		switch {
		case !tp.Reified:
		case !d.Inline:
			c.errorf(tp.P, "only type parameters of inline functions can be reified")
		case owner != nil && (owner.Interface || owner.Value):
			c.unsupported(tp.P, "reified member type parameter", "reified type parameters of members of interfaces and value classes are not supported")
		case owner != nil && !isFinal(d.Modifiers, owner):
			c.errorf(tp.P, "only type parameters of inline functions can be reified") // an open member isn't inlined
		}
	}
}

// paramSym creates the symbol of a function's or constructor's parameter
// of declared type t. A vararg parameter's type is an array of t.
func (c *Checker) paramSym(p *syntax.Param, t types.Type, decl any) *types.VarSym {
	sym := &types.VarSym{Name: p.Name, Type: t, Kind: types.Param, Decl: decl, Default: p.Default != nil,
		UnsafeVariance: p.Type != nil && p.Type.UnsafeVariance}
	if p.External {
		c.errorf(p.ExternalP, "modifier 'external' is not applicable to 'value parameter'")
	}
	if p.Vararg {
		sym.Vararg, sym.Type = true, types.VarargArray(t)
		if isUnsigned(t) && !t.N {
			// An unsigned array: UIntArray for UInt.
			sym.Type = types.InvalidType
			if a := c.u.ext.class(t.C.Name + "Array"); a != nil {
				sym.Type = types.ClassType(a)
			}
		}
	}
	return sym
}

// checkVarargs checks that at most one parameter is a vararg.
func (c *Checker) checkVarargs(ps []*syntax.Param) {
	n := 0
	for _, p := range ps {
		if p.Vararg {
			if n++; n == 2 {
				c.errorf(p.P, "multiple vararg-parameters are prohibited")
			}
		}
	}
}

// memberModifiers checks the inheritance modifiers of a declaration.
func (c *Checker) memberModifiers(m syntax.Modifiers, owner *types.ClassSym, p syntax.Pos, what string) {
	switch {
	case owner == nil && (m.Open || m.Abstract || m.Override):
		c.errorf(p, "modifiers open, abstract and override are only allowed on members of classes")
	case owner != nil && m.Abstract && !owner.Abstract:
		c.errorf(p, "abstract %s in non-abstract class '%s'", what, owner.DisplayName())
	}
}

// external reports the errors in an external function or accessor d of
// class owner, or of the top level if it is nil, and whether it has none.
// abstract is set if it or its property is abstract, declared at at.
func (c *Checker) external(d *syntax.FunDecl, owner *types.ClassSym, abstract bool, at syntax.Pos) bool {
	ok := true
	report := func(p syntax.Pos, msg string) {
		c.errorf(p, "%s", msg)
		ok = false
	}
	switch {
	case abstract:
		report(at, "external declaration cannot be abstract")
	case owner != nil && owner.Interface:
		report(d.ExternalP, "members of interfaces cannot be external")
	case d.Inline:
		report(d.ExternalP, "inline functions cannot be external")
	}
	if hasBody(d) {
		report(d.ExternalP, "external declaration cannot have a body")
	}
	return ok
}

// externalAccessors reports the errors in property d's external accessors,
// and marks those without errors external.
func (c *Checker) externalAccessors(d *syntax.PropDecl, owner *types.ClassSym) {
	reported := false // that the property is abstract
	for _, a := range []*syntax.FunDecl{d.Getter, d.Setter} {
		if a == nil || !a.External || a.Sym == nil {
			continue
		}
		ok := c.external(a, owner, d.Abstract && !reported, d.P)
		reported = true
		a.Sym.External = ok && !d.Abstract && !hasBody(a)
	}
}

// isFinal reports whether a member with these modifiers can't be
// overridden: members are final unless open, abstract, or an override not
// marked final; interface members never are.
func isFinal(m syntax.Modifiers, owner *types.ClassSym) bool {
	if owner.Interface {
		return false
	}
	return !m.Open && !m.Abstract && !(m.Override && !m.Final)
}

// privateField reports whether the private properties of class cls are
// only fields: not in interfaces, and not in value classes, whose
// properties are static methods.
func privateField(cls *types.ClassSym) bool {
	return !cls.Interface && !cls.Value
}

// jvmName returns the name that @JvmName, with the use-site target
// target, gives a declaration on the JVM, or "".
func jvmName(m *syntax.Modifiers, target string) string {
	if a := m.Find(target, "JvmName"); a != nil {
		if s, ok := a.Str("name"); ok {
			return s
		}
	}
	return ""
}

func (c *Checker) resolveProp(d *syntax.PropDecl, owner *types.ClassSym) {
	if d.Recv != nil {
		c.resolveExtProp(d)
		return
	}
	sym := d.Sym
	if sym == nil {
		sym = &types.VarSym{Name: d.Name, Kind: types.Prop, Mutable: d.Mutable, Owner: owner, Decl: d,
			Module: c.internal(&d.Modifiers, owner, ""), JVMGetter: jvmName(&d.Modifiers, "get"), JVMSetter: jvmName(&d.Modifiers, "set")}
		d.Sym = sym
	}
	if owner != nil {
		c.tparams = owner.TypeParams
		defer func() { c.tparams = nil }()
	}
	c.memberModifiers(d.Modifiers, owner, d.P, "property '"+d.Name+"'")
	c.funModifiers(d.Modifiers, d.P, "property")
	if d.Find("", "JvmStatic") != nil {
		switch {
		case owner == nil || !owner.Object || owner.Anonymous:
			c.errorf(d.P, "only members in named objects and companion objects can be annotated with '@JvmStatic'")
		case d.Field != nil:
			c.unsupported(d.P, "@JvmStatic delegated property", "@JvmStatic is not supported on delegated properties")
		case d.Override && owner.CompanionOf == nil:
			c.errorf(d.P, "override member cannot be '@JvmStatic' in an object")
		default:
			sym.JvmStatic = true
		}
	}
	if owner != nil {
		// kc doesn't support const outside objects, which is an error.
		sym.Const = d.Const && owner.Object
		sym.Private = !sym.Const && d.Visibility == syntax.Private && privateField(owner) && d.Getter == nil && d.Setter == nil && d.Field == nil
		sym.JvmField = !sym.Private && d.Find("", "JvmField") != nil && !owner.Interface && d.Getter == nil && d.Field == nil
		if sym.JvmField && sym.JvmStatic {
			c.errorf(d.P, "this annotation is not applicable to this declaration: @JvmStatic on a @JvmField property")
		}
		if sym.JvmField && owner.CompanionOf != nil && owner.CompanionOf.Interface && (d.Mutable || d.Visibility != syntax.Public) {
			c.errorf(d.P, "JvmField could be applied only if all interface companion properties are 'public final val' with '@JvmField' annotation")
		}
	}
	sym.Abstract = d.Abstract || owner != nil && owner.Interface && d.Init == nil && d.Getter == nil && d.Setter == nil
	sym.Final = owner == nil || isFinal(d.Modifiers, owner)
	if d.Field == nil {
		sym.FieldUsed = usesField(d.Getter) || usesField(d.Setter)
		// private set: no setter method, as with kotlinc.
		sym.SetterPrivate = d.Mutable && d.Setter == nil && d.SetterVis == syntax.Private && !sym.Private && !sym.JvmField
	}
	if d.SetterVis == syntax.Internal && jvmName(&d.Modifiers, "set") == "" {
		sym.SetterModule = c.internal(&syntax.Modifiers{Visibility: syntax.Internal}, owner, "")
	}
	hasField := !sym.Abstract && (d.Getter == nil || d.Mutable && d.Setter == nil || sym.FieldUsed) && len(d.Contexts) == 0
	ext := d.Getter != nil && d.Getter.External || d.Setter != nil && d.Setter.External
	if ext && !sym.Abstract && (owner == nil || !owner.Interface) {
		// kotlinc gives the property a field, which its external
		// accessors don't use: it needn't be initialized.
		sym.FieldUsed = true
	}
	if ext && owner != nil && owner.CompanionOf != nil && owner.CompanionOf.Interface &&
		(d.Find("", "JvmStatic") != nil || d.Find("get", "JvmStatic") != nil || d.Find("set", "JvmStatic") != nil) {
		c.errorf(d.P, "'@JvmStatic' annotation cannot be used on 'external' members of interface companions")
	}
	if d.Lateinit {
		c.lateinitChecks(d.P, d.Mutable, d.Init != nil, d.Getter != nil || d.Setter != nil || d.Field != nil, types.InvalidType)
		sym.Lateinit = true
	}
	switch {
	case d.Abstract && ext:
		// Reported with the accessors, by externalAccessors.
	case sym.Abstract && (d.Init != nil || d.Getter != nil || d.Setter != nil):
		c.errorf(d.P, "an abstract property cannot have an initializer or accessors")
	case owner != nil && owner.Interface && d.Init != nil:
		c.errorf(d.Init.Pos(), "property initializers are not allowed in interfaces")
	case d.Field != nil && owner != nil && owner.Interface:
		c.errorf(d.P, "delegated properties are not allowed in interfaces")
	case d.Field != nil && d.Abstract:
		c.errorf(d.P, "this property cannot be abstract and delegated")
	case owner != nil && owner.Interface && hasField:
		c.errorf(d.P, "property in an interface cannot have a backing field")
	case d.Init != nil && !hasField && !sym.FieldUsed:
		c.errorf(d.Init.Pos(), "initializer is not allowed here because this property has no backing field")
	case d.SetterVis == syntax.Private && d.Mutable && !sym.Final && !sym.Abstract && owner != nil && !owner.Final:
		c.errorf(d.P, "private setters are not allowed for open properties")
	case !sym.Abstract && d.Init == nil && hasField && !d.Lateinit:
		if owner == nil || owner.Interface {
			c.errorf(d.P, "property must be initialized or be abstract")
		}
		// Assigned in the constructor's code, as definite assignment
		// checks.
		sym.Deferred = true
	}
	if g := d.Getter; g != nil {
		g.Sym = &types.FunSym{Name: types.GetterName(d.Name), JVMName: sym.JVMGetter, Owner: owner, Decl: g, Final: sym.Final, Module: sym.Module,
			Private: privateMethod(&d.Modifiers, owner)}
		if owner == nil {
			g.Sym.Facade = c.declFacade(d.Visibility)
		}
		g.AccessorOf = sym
		sym.Getter = g.Sym
		c.composeGetter(d, sym, owner)
	}
	if st := d.Setter; st != nil {
		// Its parameter's type is the property's, once known.
		value := &types.VarSym{Name: st.Params[0].Name, Kind: types.Param, Decl: st.Params[0]}
		st.Params[0].Sym = value
		setter := d.Modifiers
		if d.SetterVis != syntax.Public {
			setter.Visibility = d.SetterVis
		}
		module := sym.Module
		if module == "" {
			module = sym.SetterModule
		}
		st.Sym = &types.FunSym{Name: types.SetterName(d.Name), JVMName: sym.JVMSetter, Owner: owner, Decl: st, Final: sym.Final, Module: module,
			Private: privateMethod(&setter, owner),
			Params:  []*types.VarSym{value}, Ret: types.UnitType, Resolved: true}
		if owner == nil {
			st.Sym.Facade = c.declFacade(setter.Visibility)
		}
		st.AccessorOf = sym
		st.Visibility = d.SetterVis
		sym.Setter = st.Sym
	}
	if g := d.Getter; g != nil && len(d.TypeParams) > 0 {
		// Those of a property with context parameters, which they use.
		g.Sym.TypeParams = c.declareTypeParams(d.TypeParams, g.Sym, false)
		c.tparams = funTypeParams(g.Sym, owner)
		c.resolveBounds(d.TypeParams)
		if d.Setter != nil {
			d.Setter.Sym.TypeParams = g.Sym.TypeParams
		}
	}
	c.propContexts(d, sym)
	c.externalAccessors(d, owner)
	switch {
	case d.Type != nil:
		sym.Type, sym.Resolved = c.resolveType(d.Type), true
		if d.Lateinit {
			c.lateinitType(d.P, sym.Type)
		}
		if d.Getter != nil {
			d.Getter.Sym.Ret, d.Getter.Sym.Resolved = sym.Type, true
			unitGetter(d.Getter.Sym)
		}
		if d.Setter != nil {
			d.Setter.Sym.Params[0].Type = sym.Type
			c.setterParamType(d)
		}
	case d.Init == nil && (d.Getter == nil || d.Getter.ExprBody == nil):
		c.errorf(d.P, "this property must have a type, be initialized, or have a getter with an expression body")
		sym.Type, sym.Resolved = types.InvalidType, true
		if d.Getter != nil {
			d.Getter.Sym.Ret, d.Getter.Sym.Resolved = types.InvalidType, true
		}
	default:
		sym.Infer = func() { c.inferProp(d) }
	}
}

// privateMethod reports whether a function or accessor declared with
// modifiers m in class owner, or at the top level if it is nil, is
// private on the JVM, as with kotlinc: an interface's too, if its bodies
// are default methods, not in DefaultImpls. kc keeps those of value
// classes public.
func privateMethod(m *syntax.Modifiers, owner *types.ClassSym) bool {
	return m.Visibility == syntax.Private && (owner == nil || !owner.Value && (!owner.Interface || owner.JVMDefault))
}

// privateCtor reports whether a constructor of class d with modifiers m
// is private on the JVM: a private one, or a sealed class's, as with
// kotlinc. An object's and an enum's are too, but only they call them.
func privateCtor(d *syntax.ClassDecl, m *syntax.Modifiers) bool {
	return !d.Object && !d.Enum && !d.Interface && (m.Visibility == syntax.Private || d.Sealed)
}

func (c *Checker) resolveClass(d *syntax.ClassDecl) {
	cls := d.Sym
	if d.Interface || d.Object || d.Enum && len(d.Secondaries()) == 0 {
		// Their secondary constructors are errors; an enum with
		// secondary constructors has only those.
		d.HasPrimary = true
	}
	if d.Enum && d.CtorMods.Explicit && d.CtorMods.Visibility != syntax.Private {
		c.errorf(d.P, "constructor must be private in enum class")
	}
	if d.CtorMods.External {
		c.errorf(d.CtorMods.ExternalP, "modifier 'external' is not applicable to 'constructor'")
	}
	ctor := &types.FunSym{Name: "<init>", Owner: cls, Ret: cls.This(), Resolved: true, Decl: d,
		Hidden: d.CtorMods.Hidden(), Private: privateCtor(d, &d.CtorMods)}
	if d.HasPrimary {
		d.Ctor = ctor
		cls.Ctor = ctor
	}
	addProp := func(p *types.VarSym, pos syntax.Pos) {
		if prev := cls.Prop(p.Name); prev != nil {
			if d, ok := p.Decl.(*syntax.PropDecl); ok && altProp(prev, d) {
				addAlt(prev, p)
			} else {
				c.errorf(pos, "conflicting declarations: %s", p.Name)
			}
		}
		cls.Props = append(cls.Props, p)
	}
	if d.Interface && len(d.CtorParams) > 0 {
		c.errorf(d.P, "an interface may not have a constructor")
	}
	if d.Object && len(d.CtorParams) > 0 {
		c.errorf(d.P, "constructors are not allowed for objects")
	}
	for _, cp := range d.CtorParams {
		c.tparams = cls.TypeParams
		cp.Sym = c.paramSym(&cp.Param, c.resolveType(cp.Type), cp)
		c.tparams = nil
		if cls.Annotation {
			c.annotationParam(cls, cp)
			// Its value, the annotation's method of the same name, which
			// returns a KClass as a java.lang.Class.
			cp.PropSym = &types.VarSym{Name: cp.Name, Type: cp.Sym.Type, Kind: types.Prop, Owner: cls, Decl: cp, Resolved: true,
				Abstract: true, JVMGetter: cp.Name}
			addProp(cp.PropSym, cp.P)
			continue
		}
		t := cp.Sym.Type
		if cp.Param.Vararg && d.Data {
			c.errorf(cp.P, "primary constructor vararg parameters are forbidden for data classes")
		}
		ctor.Params = append(ctor.Params, cp.Sym)
		cls.CtorParams = append(cls.CtorParams, cp.Sym)
		if cp.IsProp {
			cp.PropSym = &types.VarSym{Name: cp.Name, Type: t, Kind: types.Prop, Mutable: cp.Mutable, Owner: cls, Decl: cp, Resolved: true,
				Final: isFinal(cp.Modifiers, cls), Private: cp.Visibility == syntax.Private && privateField(cls),
				Module: c.internal(&cp.Modifiers, cls, ""), JVMGetter: jvmName(&cp.Modifiers, "get"), JVMSetter: jvmName(&cp.Modifiers, "set")}
			cp.PropSym.JvmField = cp.Find("", "JvmField") != nil && !cp.PropSym.Private
			c.memberModifiers(cp.Modifiers, cls, cp.P, "property '"+cp.Name+"'")
			if cp.Abstract {
				c.errorf(cp.P, "a constructor property cannot be abstract")
			}
			addProp(cp.PropSym, cp.P)
		} else if cp.Open || cp.Abstract || cp.Override {
			c.errorf(cp.P, "modifiers open, abstract and override are only allowed on properties")
		}
	}
	if cls.Value {
		c.valueParam(d)
	}
	ps := make([]*syntax.Param, len(d.CtorParams))
	for i, cp := range d.CtorParams {
		ps[i] = &cp.Param
	}
	c.checkVarargs(ps)
	c.resolveSecondaries(d)
	for _, m := range d.Members {
		switch m := m.(type) {
		case *syntax.PropDecl:
			if m.Recv != nil {
				c.declareMemberExtProp(m, cls)
				continue
			}
			c.resolveProp(m, cls)
			addProp(m.Sym, m.P)
		case *syntax.FunDecl:
			c.resolveFun(m, cls)
			if m.Recv != nil {
				if cls.ExtMethods == nil {
					cls.ExtMethods = map[string][]*types.FunSym{}
					c.u.memberExts = true
				}
				cls.ExtMethods[m.Name] = append(cls.ExtMethods[m.Name], m.Sym)
				continue
			}
			cls.Methods[m.Name] = append(cls.Methods[m.Name], m.Sym)
		}
	}
	if d.Data && !d.Object {
		c.dataMembers(d)
	}
	if d.Enum {
		c.enumMembers(d)
	}
	c.checkClassVariance(d)
}

// enumMembers declares an enum class's entries, as static fields, and its
// static methods valueOf and values.
func (c *Checker) enumMembers(d *syntax.ClassDecl) {
	cls := d.Sym
	d.EntriesList = c.u.ext.kotlin
	t := types.ClassType(cls)
	cls.Fields = map[string]*types.VarSym{}
	for _, e := range d.Entries {
		if _, dup := cls.Fields[e.Name]; dup {
			c.errorf(e.P, "conflicting declarations: %s", e.Name)
		}
		cls.Fields[e.Name] = &types.VarSym{Name: e.Name, Type: t, Kind: types.Static, Owner: cls, Resolved: true}
	}
	str := &types.VarSym{Name: "value", Type: types.StringType, Kind: types.Param}
	cls.Statics = map[string][]*types.FunSym{
		"valueOf": {{Name: "valueOf", Owner: cls, Static: true, Params: []*types.VarSym{str}, Ret: t, Resolved: true}},
		"values":  {{Name: "values", Owner: cls, Static: true, Ret: types.ArrayOf(t), Resolved: true}},
	}
}

// dataMembers declares the members that a data class gets: componentN for
// each property of the primary constructor, and copy, which takes them
// all, each defaulting to the current value. Codegen writes them, and
// equals, hashCode and toString, unless the class declares them.
func (c *Checker) dataMembers(d *syntax.ClassDecl) {
	cls := d.Sym
	if len(d.CtorParams) == 0 {
		c.errorf(d.P, "data class must have at least one primary constructor parameter")
	}
	var params []*types.VarSym
	for i, cp := range d.CtorParams {
		if !cp.IsProp {
			c.errorf(cp.P, "data class primary constructor must only have property (val / var) parameters")
			continue
		}
		params = append(params, &types.VarSym{Name: cp.Name, Type: cp.Sym.Type, Kind: types.Param, Default: true})
		name := fmt.Sprintf("component%d", i+1)
		if len(cls.Methods[name]) == 0 {
			// With the property's visibility: component2$module for an
			// internal one.
			cls.Methods[name] = []*types.FunSym{{Name: name, Owner: cls, Ret: cp.PropSym.Type, Resolved: true, Final: true, Operator: true,
				Private: privateMethod(&cp.Modifiers, cls), Protected: cp.Visibility == syntax.Protected, Module: c.internal(&cp.Modifiers, cls, "")}}
		}
	}
	if !declaresCopy(cls, len(params)) && len(params) == len(d.CtorParams) {
		cls.Methods["copy"] = append(cls.Methods["copy"], &types.FunSym{Name: "copy", Owner: cls, Params: params, Ret: cls.This(), Resolved: true, Final: true})
	}
}

// declaresCopy reports whether data class cls declares a copy with n
// parameters, which takes the place of the one the compiler would
// generate. A copy with other parameters, as GMTDate's copy(), is an
// overload of it.
func declaresCopy(cls *types.ClassSym, n int) bool {
	for _, f := range cls.Methods["copy"] {
		if len(f.Params) == n {
			return true
		}
	}
	return false
}

// defNotNull resolves an intersection type, which Kotlin allows only as
// a definitely non-nullable type, T & Any, with kotlinc's messages.
func (c *Checker) defNotNull(ref *syntax.TypeRef) types.Type {
	left := *ref
	left.And = nil
	t := c.resolveType(&left)
	and := c.resolveType(ref.And)
	switch {
	case t.K == types.Invalid || and.K == types.Invalid:
		return types.InvalidType
	case and != types.AnyType:
		c.errorf(ref.And.P, "intersection types are supported only for definitely non-nullable types: right part must be non-nullable 'Any'")
		return types.InvalidType
	case !t.IsParam() || t.N || !t.Param().NullableBound():
		c.errorf(ref.P, "intersection types are supported only for definitely non-nullable types: left part must be a type parameter with nullable bounds")
		return types.InvalidType
	}
	return t.NonNull() // T & Any
}

// checkClassVariance checks that a class's out type parameters occur only
// in out positions (property and return types) and in parameters only in
// in positions (parameter types). A var property is both.
func (c *Checker) checkClassVariance(d *syntax.ClassDecl) {
	variant := false
	for _, p := range d.Sym.TypeParams {
		variant = variant || p.Var != types.Inv
	}
	if !variant {
		return
	}
	propPos := func(mutable bool) types.Variance {
		if mutable {
			return types.Inv
		}
		return types.Out
	}
	// As in Kotlin, private members aren't checked: only this class's
	// code, which knows its type arguments, sees them.
	for _, cp := range d.CtorParams {
		if cp.IsProp && cp.Visibility != syntax.Private {
			c.checkVariance(cp.PropSym.Type, cp.Type, propPos(cp.Mutable))
		}
	}
	for _, m := range d.Members {
		switch m := m.(type) {
		case *syntax.PropDecl:
			if m.Type != nil && m.Visibility != syntax.Private {
				c.checkVariance(m.Sym.Type, m.Type, propPos(m.Mutable))
			}
		case *syntax.FunDecl:
			if m.Visibility == syntax.Private {
				continue
			}
			for _, p := range m.Params {
				c.checkVariance(p.Sym.Type, p.Type, types.In)
			}
			if m.Ret != nil {
				c.checkVariance(m.Sym.Ret, m.Ret, types.Out)
			}
		}
	}
}

var positionNames = map[types.Variance]string{types.Out: "out", types.In: "in", types.Inv: "invariant"}

// checkVariance checks type t, written as ref, in position pos. The
// parts of ref annotated @UnsafeVariance aren't checked.
func (c *Checker) checkVariance(t types.Type, ref *syntax.TypeRef, pos types.Variance) {
	c.checkVariance1(t, t, ref, pos, ref.P)
}

func (c *Checker) checkVariance1(whole, t types.Type, ref *syntax.TypeRef, pos types.Variance, at syntax.Pos) {
	if ref != nil && ref.UnsafeVariance {
		return
	}
	switch t.K {
	case types.TypeParam:
		if v := t.Param().Var; v != types.Inv && v != pos {
			c.errorf(at, "type parameter %s is declared as '%s' but occurs in '%s' position in type %s", t.Param().Name, positionNames[v], positionNames[pos], whole)
		}
	case types.Class:
		if t.A == nil {
			return
		}
		refs := argRefs(ref, len(t.A.List))
		for i, a := range t.A.List {
			v := a.V
			if v == types.Star {
				continue
			}
			if v == types.Inv && i < len(t.C.TypeParams) {
				v = t.C.TypeParams[i].Var
			}
			p := pos
			switch v {
			case types.Inv:
				p = types.Inv
			case types.In:
				switch pos {
				case types.Out:
					p = types.In
				case types.In:
					p = types.Out
				}
			}
			c.checkVariance1(whole, a.T, refs[i], p, at)
		}
	}
}

// argRefs returns the type references that type ref writes its n type
// arguments as: a function type's receiver, parameters and result, or
// nils if they aren't known.
func argRefs(ref *syntax.TypeRef, n int) []*syntax.TypeRef {
	var refs []*syntax.TypeRef
	switch {
	case ref == nil:
	case ref.Ret != nil:
		refs = append(refs, ref.Contexts...)
		if ref.Recv != nil {
			refs = append(refs, ref.Recv)
		}
		refs = append(append(refs, ref.Params...), ref.Ret)
	default:
		for _, a := range ref.Args {
			refs = append(refs, a.Type)
		}
	}
	if len(refs) != n {
		refs = make([]*syntax.TypeRef, n)
	}
	return refs
}

func (c *Checker) inferAll() {
	for _, d := range c.file.Decls {
		switch d := d.(type) {
		case *syntax.FunDecl:
			ensureFun(d.Sym)
		case *syntax.PropDecl:
			ensureVar(d.Sym)
		}
	}
	for _, d := range c.classes {
		for _, m := range d.Members {
			switch m := m.(type) {
			case *syntax.FunDecl:
				ensureFun(m.Sym)
			case *syntax.PropDecl:
				ensureVar(m.Sym)
			}
		}
	}
}

func ensureFun(f *types.FunSym) {
	if !f.Resolved && f.Infer != nil {
		f.Infer()
	}
}

func ensureVar(v *types.VarSym) {
	if !v.Resolved && v.Infer != nil {
		v.Infer()
	}
}

// inferFun checks an expression-bodied function to find its return type.
// It can run re-entrantly when one inferred function calls another.
func (c *Checker) inferFun(d *syntax.FunDecl) {
	sym := d.Sym
	if sym.InProgress {
		c.errorf(d.P, "type checking has run into a recursive problem; specify the return type of '%s'", d.Name)
		sym.Ret, sym.Resolved = types.InvalidType, true
		return
	}
	sym.InProgress = true
	saved, trial := c.state, c.trial
	c.trial = c.trialOf(sym.Owner)
	c.checkFun(d, sym.Owner)
	c.done[d] = true
	c.onUndo(func() { delete(c.done, d) })
	c.state, c.trial = saved, trial
	sym.InProgress = false
	sym.Resolved = true
}

func (c *Checker) inferProp(d *syntax.PropDecl) {
	sym := d.Sym
	if sym.InProgress {
		c.errorf(d.P, "type checking has run into a recursive problem; specify the type of '%s'", d.Name)
		sym.Type, sym.Resolved = types.InvalidType, true
		return
	}
	sym.InProgress = true
	saved, trial := c.state, c.trial
	c.trial = c.trialOf(sym.Owner)
	if d.Getter != nil && d.Init == nil {
		// val x get() = ...: the type is the getter's.
		c.inferFun(d.Getter)
		sym.Type = d.Getter.Sym.Ret
		unitGetter(d.Getter.Sym)
	} else {
		c.checkPropInit(d, sym.Owner)
		if d.Getter != nil {
			d.Getter.Sym.Ret, d.Getter.Sym.Resolved = sym.Type, true
			unitGetter(d.Getter.Sym)
		}
	}
	if d.Setter != nil {
		d.Setter.Sym.Params[0].Type = sym.Type
		c.setterParamType(d)
	}
	c.done[d] = true
	c.onUndo(func() { delete(c.done, d) })
	c.state, c.trial = saved, trial
	sym.InProgress = false
	sym.Resolved = true
}

// checkBodies checks everything not already checked during inference.
func (c *Checker) checkBodies() {
	c.annotations(c.file.Annotations, nil)
	for _, d := range c.file.Decls {
		c.declAnnotations(d, nil)
	}
	for _, d := range c.classes {
		if d.Sym != nil && d.Sym.Outer == nil {
			continue // a top-level class, checked above
		}
		c.declAnnotations(d, d.Sym.Outer)
	}
	for _, d := range c.file.Decls {
		switch d := d.(type) {
		case *syntax.FunDecl:
			c.checkDefaults(d.Params, nil, d.Sym)
			if !c.done[d] && (d.Body != nil || d.ExprBody != nil) {
				c.checkFun(d, nil)
			}
		case *syntax.PropDecl:
			c.checkProp(d, nil)
		}
	}
	for _, d := range c.classes {
		c.checkClass(d)
	}
	c.propRefs()
	c.inlineCycles()
}

// checkClass checks a class's code: its constructors' calls, property
// initializers, init blocks and members' bodies.
func (c *Checker) checkClass(d *syntax.ClassDecl) {
	{
		c.checkInheritance(d)
		if d.Fun {
			c.checkFunInterface(d)
		}
		for _, e := range d.Entries {
			if e.Body != nil {
				continue // its class's superclass constructor call
			}
			// An entry calls the enum's constructor. Its arguments see the
			// enum's nested classes and entries, but not this.
			c.state = state{scope: &scope{}, tparams: d.Sym.TypeParams, ctorArgs: true, staticCls: d.Sym}
			c.lookupCls = d.Sym
			e.Call.Ctor = d.Sym
			var cands []callee
			for _, f := range userCtors(d.Sym) {
				cands = append(cands, callee{f: f})
			}
			c.resolve(e.Call, cands, noWant, nil)
			c.lookupCls = nil
		}
		params := make([]*syntax.Param, len(d.CtorParams))
		for i, cp := range d.CtorParams {
			params[i] = &cp.Param
		}
		if d.Ctor != nil {
			c.checkDefaults(params, d.Sym, d.Ctor)
		}
		if d.SuperCall != nil {
			c.superCtor(d)
		}
		c.checkDelegates(d)
		c.checkSecondaries(d)
		for _, m := range d.Members {
			switch m := m.(type) {
			case *syntax.FunDecl:
				c.checkDefaults(m.Params, d.Sym, m.Sym)
				if !c.done[m] && (m.Body != nil || m.ExprBody != nil) {
					c.checkFun(m, d.Sym)
				}
			case *syntax.PropDecl:
				c.checkProp(m, d.Sym)
			case *syntax.InitBlock:
				c.state = state{scope: c.ctorScope(d.Sym), cls: d.Sym, this: newThis(d.Sym), tparams: d.Sym.TypeParams, init: true}
				c.inAnon(d.Sym)
				c.block(m.Body, noWant, false)
			}
		}
		c.classAssignment(d)
	}
}

// checkDefaults checks the default values of a function's or a
// constructor's parameters. Each can use the parameters before it; in a
// method, it can use the class's members too.
func (c *Checker) checkDefaults(params []*syntax.Param, cls *types.ClassSym, f *types.FunSym) {
	s := &scope{}
	ctor := f.Name == "<init>"
	c.state = state{scope: s, tparams: funTypeParams(f, cls), defaultsOf: f}
	if ctor {
		// A constructor's default values see the class's nested classes
		// and companion, but not this.
		c.staticCls, c.lookupCls = cls, cls
		defer func() { c.lookupCls = nil }()
	} else {
		c.cls, c.this = cls, newThis(cls)
	}
	if d, ok := f.Decl.(*syntax.FunDecl); ok {
		c.recvs, c.ctxs = extRecvs(d), funContexts(d)
		declareNamedContexts(s, d)
	}
	c.inAnon(cls)
	for i, p := range params {
		if p.Default != nil {
			c.defaultParam = i
			c.assignable(c.value(p.Default, p.Sym.Type), p.Sym.Type, p.Default.Pos())
		}
		s.declare(p.Sym)
	}
}

// inheritDefaults gives overrides the default values of the methods they
// override: Kotlin doesn't allow an override to declare its own. Overrides
// also inherit the operator and infix modifiers, and the mangling of
// internal members' names.
// builtinOperators are the operator functions of Kotlin's collection types
// and CharSequence, by their Java names: get, MutableList's set, contains
// and iterator, and an Iterator's next and hasNext. CharSequence's get is
// Java's charAt.
var builtinOperators = map[string]bool{"get": true, "set": true, "contains": true, "iterator": true, "next": true, "hasNext": true, "charAt": true}

func (c *Checker) inheritDefaults() {
	for _, d := range c.classes {
		c.inheritModule(d)
		for _, m := range d.Members {
			fd, ok := m.(*syntax.FunDecl)
			if !ok || !fd.Override {
				continue
			}
			for _, p := range fd.Params {
				if p.Default != nil {
					c.errorf(p.Default.Pos(), "an overriding function is not allowed to specify default values for its parameters")
				}
			}
			for _, o := range c.overridden(d.Sym, fd.Sym) {
				// As in Kotlin, an override of an operator or infix
				// function is one too.
				fd.Sym.Operator = fd.Sym.Operator || o.Operator || o.Owner != nil && o.Owner.Name == charSequence && o.Name == "charAt"
				fd.Sym.Infix = fd.Sym.Infix || o.Infix
				// For metadata: an override of a Kotlin operator, a
				// function type's invoke among them, or of Any.equals,
				// which is Object.equals here.
				kotlinOp := o.Owner != nil && (o.Owner.External && o.Owner.KotlinName != "" || o.Owner.Func) && o.Operator
				if o.Owner != nil && (o.Owner.Java != nil || o.Owner.Name == charSequence) {
					// A Java method of a Kotlin collection type or
					// CharSequence: an operator if Kotlin's is.
					// MutableList's set(index, element), not
					// MutableListIterator's set(element).
					kotlinOp = builtinOperators[o.Name] && (o.Name != "set" || len(o.Params) == 2)
				}
				if od, ok := o.Decl.(*syntax.FunDecl); ok {
					kotlinOp = od.Operator || o.KotlinOperator
				}
				if kotlinOp || (o.Name == "equals" || o.Name == "compareTo") && len(o.Params) == 1 {
					fd.Sym.KotlinOperator = true
				}
				if !fd.Explicit && fd.Sym.Inherited == 0 {
					switch od, _ := o.Decl.(*syntax.FunDecl); {
					case o.Protected || od != nil && od.Visibility == syntax.Protected:
						fd.Sym.Inherited = types.InheritProtected
						if ow := o.Owner; ow != nil && ow.External && ow.JVMMethods[o.JVM()+types.FunDesc(o)]&classfile.AccPublic != 0 {
							fd.Sym.PublicOnJVM = true
						}
					case o.Module != "" || od != nil && od.Visibility == syntax.Internal:
						fd.Sym.Inherited = types.InheritInternal
					}
				}
				// An override declared without a visibility has the
				// overridden function's: an internal one's name is
				// mangled.
				if !fd.Explicit && fd.Sym.JVMName == "" && o.Module != "" {
					fd.Sym.Module = o.Module
				}
				// A function of the program with default values, or one of
				// the class path's.
				c.inheritExtDefaults(o)
				od, ok := o.Decl.(*syntax.FunDecl)
				if ok && !funDeclaresDefaults(od) || !ok && (o.Decl != nil || !declaresDefaults(o)) {
					continue
				}
				for i, p := range fd.Sym.Params {
					if fd.Inline && o.Params[i].Default && !p.Default {
						c.errorf(fd.Params[i].P, "functional parameters with inherited default values are not yet supported in inline functions")
					}
					p.Default = p.Default || o.Params[i].Default
				}
				fd.Sym.DefaultsOf = o
			}
		}
	}
}

// inheritModule gives the properties of class d that override internal
// properties the mangling of their accessors' JVM names, as with
// kotlinc: an override declared without a visibility has the overridden
// property's.
func (c *Checker) inheritModule(d *syntax.ClassDecl) {
	inherit := func(p *types.VarSym, m *syntax.Modifiers) {
		if m.Explicit || p.Module != "" {
			return
		}
		d.Sym.Supertypes(func(s *types.ClassSym) bool {
			if s != d.Sym && !s.External {
				if sp := s.Prop(p.Name); sp != nil && sp.Module != "" {
					p.Module = sp.Module
					if p.Getter != nil {
						p.Getter.Module = sp.Module
					}
					if p.Setter != nil {
						p.Setter.Module = sp.Module
					}
				}
			}
			return p.Module == ""
		})
	}
	for _, cp := range d.CtorParams {
		if cp.PropSym != nil && cp.Override {
			inherit(cp.PropSym, &cp.Modifiers)
		}
	}
	for _, m := range d.Members {
		if pd, ok := m.(*syntax.PropDecl); ok && pd.Override && pd.Recv == nil && pd.Sym != nil {
			inherit(pd.Sym, &pd.Modifiers)
		}
	}
}

// funDeclaresDefaults reports whether a function declares default values.
func funDeclaresDefaults(d *syntax.FunDecl) bool {
	for _, p := range d.Params {
		if p.Default != nil {
			return true
		}
	}
	return false
}

// checkProp checks a property's initializer or getter, unless inference
// already has.
func (c *Checker) checkProp(d *syntax.PropDecl, cls *types.ClassSym) {
	if d.Init != nil && !c.done[d] {
		c.checkPropInit(d, cls)
	}
	if d.Getter != nil && !c.done[d.Getter] && hasBody(d.Getter) {
		c.checkFun(d.Getter, cls)
	}
	if d.Setter != nil && d.Setter.Sym != nil && hasBody(d.Setter) { // nil: an extension property's without a getter
		c.checkFun(d.Setter, cls)
	}
}

// hasBody reports whether a function or an accessor has a body: an
// abstract extension property's accessors don't.
func hasBody(d *syntax.FunDecl) bool { return d.Body != nil || d.ExprBody != nil }

// lateinitChecks reports the uses of lateinit that Kotlin forbids, on a
// property or a local var. t is the type, or Invalid if it isn't known
// yet: see lateinitType.
func (c *Checker) lateinitChecks(p syntax.Pos, mutable, init, accessors bool, t types.Type) {
	switch {
	case !mutable:
		c.errorf(p, "'lateinit' modifier is allowed only on mutable properties")
	case init:
		c.errorf(p, "'lateinit' modifier is not allowed on properties with an initializer")
	case accessors:
		c.errorf(p, "'lateinit' modifier is not allowed on properties with a custom getter or setter, or delegated ones")
	default:
		c.lateinitType(p, t)
	}
}

// lateinitType checks the type of a lateinit property: neither nullable
// nor primitive, nor a type parameter that can be nullable.
func (c *Checker) lateinitType(p syntax.Pos, t types.Type) {
	switch {
	case t.K == types.Invalid:
	case types.MayBeNull(t):
		c.errorf(p, "'lateinit' modifier is not allowed on properties of nullable types")
	case t.IsPrimitive() || types.Unboxed(t):
		c.errorf(p, "'lateinit' modifier is not allowed on properties of primitive types")
	}
}

// usesField reports whether a property's accessor reads or assigns its
// backing field, field. Like Kotlin, it goes by the name.
func usesField(a *syntax.FunDecl) bool {
	if a == nil {
		return false
	}
	found := false
	f := func(n any) bool {
		if id, ok := n.(*syntax.Ident); ok && id.Name == "field" {
			found = true
		}
		return !found
	}
	if a.Body != nil {
		syntax.Inspect(a.Body, f)
	}
	if a.ExprBody != nil {
		syntax.Inspect(a.ExprBody, f)
	}
	return found
}

// setterParamType checks the type written on a setter's parameter, set(v:
// T), which must be the property's.
func (c *Checker) setterParamType(d *syntax.PropDecl) {
	p := d.Setter.Params[0]
	if p.Type == nil || d.Field != nil {
		return
	}
	if t := c.resolveType(p.Type); t.K != types.Invalid && t != d.Sym.Type {
		c.errorf(p.P, "setter parameter type must be equal to the type of the property, i.e. '%s'", d.Sym.Type)
	}
}

// ctorScope makes primary constructor parameters visible, as they are in
// property initializers and init blocks.
func (c *Checker) ctorScope(cls *types.ClassSym) *scope {
	s := &scope{}
	for _, p := range cls.CtorParams {
		s.declare(p)
	}
	return s
}

// mainArgs reports whether a function's only parameter is an
// Array<String>, as main's args.
func mainArgs(sym *types.FunSym) bool {
	if len(sym.Params) != 1 {
		return false
	}
	d, ok := types.Desc(sym.Params[0].Type)
	return ok && d == "[Ljava/lang/String;" && !sym.Params[0].Type.N
}

func (c *Checker) checkFun(d *syntax.FunDecl, cls *types.ClassSym) {
	c.pluginSerializer(d)
	c.parcelBody(d, cls)
	if d.Inline {
		defer c.publicInline(d, cls)
	}
	sym := d.Sym
	s := &scope{}
	if p := d.AccessorOf; p != nil && p.HasField() {
		// An accessor's field is its property's backing field.
		s.declare(&types.VarSym{Name: "field", Kind: types.BackingField, Type: p.Type, Mutable: p.Mutable, Prop: p, Resolved: true})
	}
	declareNamedContexts(s, d)
	for _, p := range sym.Params {
		s.declare(p)
	}
	c.state = state{scope: s, cls: cls, this: newThis(cls), fn: sym, retType: sym.Ret, retAllowed: true, tparams: funTypeParams(sym, cls),
		recvs: extRecvs(d), ctxs: funContexts(d)}
	c.inAnon(cls)
	if sym.Suspend {
		d.Continuation = c.anonName()
	}
	if sym.Composable {
		defer c.composeFunFlow(d)
	}
	deferred := c.deferred
	defer func() {
		if c.deferred != deferred {
			c.funAssignment(d) // only deferred locals are tracked
		}
	}()
	if sym.Suspend && cls == nil && d.Name == "main" && d.Recv == nil && (len(d.Params) == 0 || mainArgs(sym)) {
		defer func() { d.Entry = c.anonName() }()
	}
	if d.Tailrec {
		defer c.tailCalls(d)
		if cls != nil && !sym.Final {
			c.errorf(d.P, "tailrec is not allowed on open members")
		}
	}
	if d.Body != nil {
		t := c.block(d.Body, noWant, false)
		if sym.Ret.K != types.Unit && sym.Ret.K != types.Invalid && t.K != types.Nothing {
			c.errorf(d.Body.P, "missing return statement in function '%s'", d.Name)
		}
		return
	}
	if sym.Resolved {
		c.assignable(c.value(d.ExprBody, sym.Ret), sym.Ret, d.ExprBody.Pos())
		return
	}
	c.retAllowed = false // `return` needs an explicit return type
	t := c.expr(d.ExprBody, noWant)
	if t.K == types.Range {
		c.errorf(d.ExprBody.Pos(), "ranges are only supported in 'for' loops and 'in' checks")
		t = types.InvalidType
	}
	if sym.Ret = t; !keepsAnon(&d.Modifiers, cls) {
		sym.Ret = approx(t)
	}
}

// unitGetter marks getter g of a property as kotlinc's: a Unit property's
// returns the kotlin.Unit object.
func unitGetter(g *types.FunSym) {
	if g.Ret.K == types.Unit {
		g.BoxedRet = true
	}
}

func (c *Checker) checkPropInit(d *syntax.PropDecl, cls *types.ClassSym) {
	c.state = state{scope: &scope{}, cls: cls, this: newThis(cls), init: cls != nil, declName: d.Name}
	if cls != nil {
		c.scope = c.ctorScope(cls)
		c.tparams = cls.TypeParams
	}
	c.inAnon(cls)
	sym := d.Sym
	if sym.Resolved {
		c.assignable(c.value(d.Init, sym.Type), sym.Type, d.Init.Pos())
		return
	}
	var t types.Type
	provided := noWant
	if want := c.delegateWant(d); want.K == types.Invalid && delegateTyped(d.DelegateOf) {
		p := d.DelegateOf
		owner := types.NothingType.Nullable()
		if p.Ref != nil && p.Ref.Recv {
			owner = p.Sym.Owner.This()
		}
		t, provided = c.inferDelegate(d.Init, p.Sym.Type, owner, c.delegateThis(p), p.Mutable)
	} else {
		t = c.value(d.Init, want)
	}
	if p := d.DelegateOf; p != nil && p.Ref != nil && t.K != types.Invalid {
		t = c.provideDelegate(d, p, t, provided)
	}
	if sym.Type = t; !keepsAnon(&d.Modifiers, cls) {
		sym.Type = approx(t)
	}
}

// provideDelegate makes the delegate of property p, the initializer of
// its field d of type t, the result of t's provideDelegate operator, if it
// has one: d.provideDelegate(thisRef, property), as clikt's options and
// arguments register themselves with their command. Its result is
// expected to be of type want.
func (c *Checker) provideDelegate(d, p *syntax.PropDecl, t, want types.Type) types.Type {
	pos := d.Init.Pos()
	var this syntax.Expr
	if p.Ref.Recv {
		t := &syntax.This{}
		t.P = pos
		this = t
	} else {
		n := &syntax.NullLit{}
		n.P = pos
		this = n
	}
	m := &syntax.Member{X: checked(d.Init, t), Name: "provideDelegate"}
	m.P = pos
	call := &syntax.Call{Fun: m, Args: []syntax.Expr{this, p.Ref}, Operator: true}
	call.P = pos
	if !c.hasFun(t, "provideDelegate", call) {
		return t
	}
	d.Init = call
	return c.value(call, want)
}

// delegateWant returns the type a delegated property's delegate is
// expected to have, for its type arguments to be inferred from the
// property's type: Lazy<T> for val x: T by lazy { ... }. For other
// delegates, it returns noWant.
func (c *Checker) delegateWant(d *syntax.PropDecl) types.Type {
	p := d.DelegateOf
	if p != nil {
		if t := c.kpropDelegateWant(d.Init); t.K != types.Invalid {
			return t
		}
	}
	if !delegateTyped(p) {
		return noWant
	}
	return c.delegateExprWant(d.Init, p.Sym.Type, c.delegateThis(p))
}

// delegateTyped reports whether delegated property p's type is declared.
func delegateTyped(p *syntax.PropDecl) bool {
	return p != nil && p.Sym != nil && p.Sym.Resolved && p.Type != nil && p.Sym.Type.K != types.Invalid
}

// delegateThis returns the type of the thisRef that delegated property p
// passes its delegate: its receiver's, or null for a top-level property.
func (c *Checker) delegateThis(p *syntax.PropDecl) types.Type {
	if p.Getter != nil && p.Getter.Sym != nil && p.Getter.Sym.Recv.K != types.Invalid {
		return p.Getter.Sym.Recv
	}
	if owner := p.Sym.Owner; owner != nil {
		return owner.This()
	}
	return types.NothingType.Nullable()
}

// kpropDelegateWant returns KProperty<*> if delegate x is a property
// reference, val y by ::x, which is then a KProperty object, whose
// getValue and setValue are the standard library's, or else noWant.
func (c *Checker) kpropDelegateWant(x syntax.Expr) types.Type {
	if l, ok := x.(*syntax.Lambda); !ok || l.Ref == nil {
		return noWant
	}
	cls := c.u.ext.class("kotlin/reflect/KProperty")
	if cls == nil {
		return noWant
	}
	return types.GenericType(cls, []types.Arg{{V: types.Star}})
}

// delegateExprWant returns the type that delegate x of a property of type
// t, in a class whose this is of type this, is expected to have.
func (c *Checker) delegateExprWant(x syntax.Expr, t, this types.Type) types.Type {
	call, ok := x.(*syntax.Call)
	if !ok {
		return noWant
	}
	id, ok := call.Fun.(*syntax.Ident)
	if !ok {
		return noWant
	}
	if cls := c.findClass(id.Name); cls != nil && len(cls.TypeParams) > 0 && id.Name != "lazy" {
		// A generic class's constructor, of a ReadOnlyProperty (or a
		// ReadWriteProperty): its type arguments are inferred from the
		// property's type, as for clikt's NullableLateinit<T>.
		rop := c.u.ext.class("kotlin/properties/ReadOnlyProperty")
		if rop == nil {
			return noWant
		}
		found := false
		cls.Supertypes(func(s *types.ClassSym) bool {
			found = s == rop
			return !found
		})
		if !found {
			return noWant
		}
		return types.GenericType(rop, []types.Arg{{T: this}, {T: t}})
	}
	if l := c.passedThrough(call, id.Name); l != nil {
		// remember { mutableStateOf(null) }: the lambda's result is.
		if n := len(l.Body.Stmts); n > 0 {
			if e, ok := l.Body.Stmts[n-1].(*syntax.ExprStmt); ok {
				return c.delegateExprWant(e.X, t, this)
			}
		}
		return noWant
	}
	if id.Name != "lazy" {
		return c.delegateFunWant(id.Name, t)
	}
	lazy := c.u.ext.class("kotlin/Lazy")
	if lazy == nil {
		return noWant
	}
	return types.GenericType(lazy, []types.Arg{{T: t}})
}

// passedThrough returns call's trailing lambda if every function with its
// name returns what the lambda returns: the type parameter T of Compose's
// remember(calculation: () -> T): T.
func (c *Checker) passedThrough(call *syntax.Call, name string) *syntax.Lambda {
	if !call.Trailing() {
		return nil
	}
	l, ok := call.Args[len(call.Args)-1].(*syntax.Lambda)
	if !ok || l.Ref != nil {
		return nil
	}
	fs := append(c.topFuns(name), c.kotlinFuns(name, false)...)
	if len(fs) == 0 {
		return nil
	}
	for _, f := range fs {
		ensureFun(f)
		if !f.Ret.IsParam() || !slices.Contains(f.TypeParams, f.Ret.Param()) || len(f.Params) == 0 {
			return nil
		}
		ft := f.Params[len(f.Params)-1].Type
		if !ft.IsFunc() {
			return nil
		}
		r := ft.A.Get(ft.A.Len() - 1).T
		if !r.IsParam() || r.Param() != f.Ret.Param() {
			return nil
		}
	}
	return l
}

// delegateFunWant returns the type that a call of the generic functions
// with a name is expected to have as the delegate of a property of type
// t: C<t>, if they return a C<T> for their type parameter T and C's
// getValue returns its type parameter. ktor's var interceptors: List<...>?
// by atomic(null) is an AtomicRef<List<...>?>, not an AtomicRef<Nothing?>.
// A property of a primitive type is left to the overloads that take one,
// if there are any: atomic(0) for an AtomicInt.
func (c *Checker) delegateFunWant(name string, t types.Type) types.Type {
	if t.K == types.Invalid {
		return noWant
	}
	var out *types.ClassSym
	plain := false // an overload that isn't generic
	fs := append(c.topFuns(name), c.kotlinFuns(name, false)...)
	if c.cls != nil {
		// Extensions of this: a Fragment's viewModels().
		for _, cl := range c.extCallees(name, c.cls.This()) {
			fs = append(fs, cl.f)
		}
	}
	lazy := c.u.ext.class("kotlin/Lazy")
	for _, f := range fs {
		ensureFun(f)
		r := f.Ret
		if r.K != types.Class || r.A.Len() != 1 || !r.A.Get(0).T.IsParam() || !slices.Contains(f.TypeParams, r.A.Get(0).T.Param()) {
			plain = plain || len(f.TypeParams) == 0
			continue
		}
		cls := r.C
		cls.Ensure()
		if len(cls.TypeParams) != 1 {
			continue
		}
		// getValue returns the class's type parameter: a member's, or
		// Lazy's extension for a Lazy of it.
		ofLazy := false
		if lazy != nil {
			l, ok := c.asSuper(types.GenericType(cls, []types.Arg{{T: types.ParamType(cls.TypeParams[0])}}), lazy)
			ofLazy = ok && l.A.Get(0).T.IsParam() && l.A.Get(0).T.Param() == cls.TypeParams[0]
		}
		if !ofLazy && !slices.ContainsFunc(c.javaMethods(cls, "getValue"), func(g *types.FunSym) bool {
			ensureFun(g)
			return len(g.Params) == 2 && g.Ret.IsParam() && g.Ret.Param() == cls.TypeParams[0]
		}) && !c.extGetsParam(cls) {
			continue
		}
		if out != nil && out != cls {
			return noWant
		}
		out = cls
	}
	if out == nil || plain && t.IsPrimitive() {
		return noWant
	}
	return types.GenericType(out, []types.Arg{{T: t}})
}

// extGetsParam reports whether an extension getValue in scope returns
// generic class cls's type parameter: Compose's State<T>.getValue, for
// a MutableState<T>.
func (c *Checker) extGetsParam(cls *types.ClassSym) bool {
	p := cls.TypeParams[0]
	t := types.GenericType(cls, []types.Arg{{T: types.ParamType(p)}})
	return slices.ContainsFunc(c.extCallees("getValue", t), func(cl callee) bool {
		g := cl.f
		ensureFun(g)
		if len(g.Params) != 2 || !g.Ret.IsParam() || g.Recv.K != types.Class || g.Recv.A.Len() != 1 {
			return false
		}
		a := g.Recv.A.Get(0).T
		if !a.IsParam() || a.Param() != g.Ret.Param() {
			return false
		}
		s, ok := c.asSuper(t, g.Recv.C)
		return ok && s.A.Len() == 1 && s.A.Get(0).T.IsParam() && s.A.Get(0).T.Param() == p
	})
}

// funTypeParams lists the type parameters visible in a function: its own,
// then its class's.
func funTypeParams(f *types.FunSym, cls *types.ClassSym) []*types.TypeParamSym {
	if cls == nil || len(cls.TypeParams) == 0 {
		return f.TypeParams
	}
	return append(f.TypeParams[:len(f.TypeParams):len(f.TypeParams)], cls.TypeParams...)
}

func (c *Checker) assignable(from, to types.Type, p syntax.Pos) {
	if !c.subtype(from, to) {
		c.errorf(p, "type mismatch: inferred type is %s but %s was expected", from, to)
	}
}

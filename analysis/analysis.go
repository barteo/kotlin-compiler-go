// Package analysis runs kc's checker on parsed psi snapshots and reports
// what it resolved as facts keyed by psi node: the types of expressions,
// the symbols that references and declarations name, the calls that
// expressions make, the exhaustiveness of when expressions, and which
// expressions are used as values or unreachable.
//
// It lowers each snapshot's tree to kc's AST directly, recording which
// psi node each AST node comes from, checks the AST as the compiler
// does, and walks it back to the psi nodes. Code that kc can't compile
// is reported as diagnostics, with no facts: a caller must not act on
// partial results.
//
// Not every fact family is computed yet. Data.Capabilities lists those
// that are; a fact field outside them is always zero.
package analysis

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/psi"
)

// Config describes the module the snapshots belong to.
type Config struct {
	// JDKHome is the JDK whose classes the sources may use. If it is
	// empty and NoJDK isn't set, the JDK is looked for as kc does.
	JDKHome string
	// ModuleName is the module's name, which internal declarations'
	// JVM names end with.
	ModuleName string
	// ClassPath are the jars and class directories the sources compile
	// against. The Kotlin standard library is not added: callers that
	// want it name it here.
	ClassPath []string
	// NoJDK analyzes without JDK classes.
	NoJDK bool
	// Jobs is how many goroutines lower and check the sources; less than
	// 1 means 1.
	Jobs int
	// ExplicitAPIMode is kotlinc's -Xexplicit-api: "", "disable",
	// "warning" or "strict". It is validated; kc's checker doesn't
	// enforce it.
	ExplicitAPIMode string
}

// ErrUnsupported is returned, wrapped, for input kc can't analyze at
// all, such as scripts.
var ErrUnsupported = errors.New("analysis: unsupported input")

// ElementID names a psi node of a snapshot.
type ElementID struct {
	Snapshot psi.SnapshotID `json:"snapshot"`
	Node     psi.NodeID     `json:"node"`
}

// Diagnostic is an error that stops analysis: a source error, with Code
// "source_error", or a construct kc doesn't support, with Code
// "unsupported:" and the feature's name. Line and Column are 1-based;
// columns count bytes.
type Diagnostic struct {
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Type is a resolved Kotlin type.
type Type struct {
	// Name is the classifier's fully qualified name, dotted, as
	// kotlin.collections.List, with Kotlin's names for the Java classes
	// it maps, kotlin.Any for java.lang.Object. A type parameter has its
	// own name.
	Name string `json:"name"`
	// Nullable is set on a type that holds null: T?, or a type parameter
	// whose bound is nullable. MarkedNullable is set when it is written
	// with ?.
	Nullable       bool `json:"nullable,omitempty"`
	MarkedNullable bool `json:"markedNullable,omitempty"`
	// Platform is set on a Java type of unknown nullability, T!.
	Platform bool `json:"platform,omitempty"`
	// Parameter is set on a type parameter, declared at ParameterOwner
	// if that is in the sources.
	Parameter      bool       `json:"parameter,omitempty"`
	ParameterOwner *ElementID `json:"parameterOwner,omitempty"`
	// SAM is set on a fun interface or a Java interface with a single
	// abstract method, Suspend on a suspend function type.
	SAM     bool `json:"sam,omitempty"`
	Suspend bool `json:"suspend,omitempty"`
	// Projection is a type argument's projection: "", "in", "out" or
	// "*".
	Projection string `json:"projection,omitempty"`
	Arguments  []Type `json:"arguments,omitempty"`
	// Supertypes are the fully qualified names of the classifier's
	// supertypes, direct and indirect; for a Java class Kotlin maps, its
	// Java name too: kotlin.Throwable and java.lang.Throwable.
	Supertypes []string `json:"supertypes,omitempty"`
}

// Symbol is a declaration: a class, function, constructor, property,
// local variable or parameter.
type Symbol struct {
	ID uint32 `json:"id"`
	// Kind is "class", "function", "constructor", "property", "local" or
	// "parameter".
	Kind string `json:"kind"`
	// Name is a class's JVM binary name, a/b/C$D, or another symbol's
	// simple name.
	Name string `json:"name"`
	// QualifiedName is the fully qualified name of a class, a top-level
	// or member function or property; "" for locals and parameters.
	QualifiedName string `json:"qualifiedName,omitempty"`
	// Owner is the JVM binary name of a member's class.
	Owner string `json:"owner,omitempty"`
	// Type is a variable's type, a function's return type, or a class's
	// own type.
	Type Type `json:"type"`
	// Declaration is the psi node that declares the symbol, if it is in
	// the sources.
	Declaration *ElementID `json:"declaration,omitempty"`
	// Supertypes are a class's supertypes' fully qualified names.
	Supertypes []string `json:"supertypes,omitempty"`
	// Shadowed is set on a local or parameter that hides another.
	Shadowed bool `json:"shadowed,omitempty"`
	// HasAbstractMembers and HasConcreteMembers report whether a class's
	// member scope, as kotlinc's unsubstitutedMemberScope, holds an
	// abstract member, or one that isn't: the members of the class and its
	// supertypes that nothing in the hierarchy overrides, kotlin.Any's open
	// ones and the nested classes. HasAbstractSuperclass is set when the
	// superclass is abstract.
	HasAbstractMembers    bool `json:"hasAbstractMembers,omitempty"`
	HasConcreteMembers    bool `json:"hasConcreteMembers,omitempty"`
	HasAbstractSuperclass bool `json:"hasAbstractSuperclass,omitempty"`
}

// Call is a resolved call of a function or constructor.
type Call struct {
	// Name is the callee's fully qualified name: pkg.f, pkg.C.m, or
	// pkg.C.<init> for a constructor.
	Name string `json:"name"`
	// Declaration is the callee's declaration, if it is in the sources.
	Declaration *ElementID `json:"declaration,omitempty"`
	// Extension is set on a call of an extension function, whose
	// receiver's declared type is DeclaredReceiver. Receiver is the type
	// of the receiver the call passes, explicit or implicit, and
	// ImplicitReceiver the declaration that provides an implicit one.
	Extension        bool       `json:"extension,omitempty"`
	DeclaredReceiver *Type      `json:"declaredReceiver,omitempty"`
	Receiver         *Type      `json:"receiver,omitempty"`
	ImplicitReceiver *ElementID `json:"implicitReceiver,omitempty"`
	// Parameters are the callee's value parameters' types, as declared,
	// with ParameterNames and Varargs.
	Parameters     []Type   `json:"parameters,omitempty"`
	ParameterNames []string `json:"parameterNames,omitempty"`
	Varargs        []bool   `json:"varargs,omitempty"`
	// ArgumentMapping gives the parameter each argument is passed to,
	// by argument: the value arguments in order, then a trailing lambda.
	ArgumentMapping []int `json:"argumentMapping,omitempty"`
	// ReturnType is the call's type, DeclaredReturnType the callee's.
	ReturnType         *Type `json:"returnType,omitempty"`
	DeclaredReturnType *Type `json:"declaredReturnType,omitempty"`
	// TypeParameters is the callee's number of type parameters.
	TypeParameters     int      `json:"typeParameters,omitempty"`
	TypeParameterNames []string `json:"typeParameterNames,omitempty"`
	// Annotations are the fully qualified names of the callee's
	// annotations, its class's (not an enclosing class's) and its Java
	// package's.
	Annotations        []string `json:"annotations,omitempty"`
	OwnerAnnotations   []string `json:"ownerAnnotations,omitempty"`
	PackageAnnotations []string `json:"packageAnnotations,omitempty"`
	// Flags of the callee. FromJava is set on a Java method or
	// constructor, whose parameter names aren't stable (StableNames).
	Suspend     bool `json:"suspend,omitempty"`
	Inline      bool `json:"inline,omitempty"`
	Operator    bool `json:"operator,omitempty"`
	FromJava    bool `json:"fromJava,omitempty"`
	StableNames bool `json:"stableNames,omitempty"`
	// Crossinline and Noinline are an inline callee's parameters' flags.
	Crossinline []bool `json:"crossinline,omitempty"`
	Noinline    []bool `json:"noinline,omitempty"`
	// ValueInvocation is set on a call of a value of a function type.
	ValueInvocation bool `json:"valueInvocation,omitempty"`
	// Targets are other functions a call may resolve to.
	Targets []Call `json:"targets,omitempty"`
}

// When describes a when expression.
type When struct {
	// Exhaustive is set when the branches cover every value of the
	// subject without the else branch, if there is one: an else branch
	// of an exhaustive when is redundant.
	Exhaustive bool `json:"exhaustive"`
	// Category is the subject's kind when exhaustiveness is checked
	// against its cases: "enum class", "sealed class", "boolean", or "".
	Category string `json:"category,omitempty"`
	// Missing are the cases the branches other than else don't cover,
	// as kotlinc names them: an enum entry B, a subclass is S, an object
	// O, true, null.
	Missing []string `json:"missing,omitempty"`
}

// Fact is what analysis knows of a psi node.
type Fact struct {
	Element ElementID `json:"element"`
	// Role is "declaration", "reference" or "expression".
	Role string `json:"role,omitempty"`
	// Type is an expression's type; for a reference to a variable that
	// a smart cast makes non-null, the variable's declared type, with
	// FlowNonNull and DefinitelyNonNull set.
	Type *Type `json:"type,omitempty"`
	// Symbol is the ID of the symbol a declaration declares or a
	// reference names; 0 for none.
	Symbol uint32 `json:"symbol,omitempty"`
	// Call is the call an expression or a callee reference makes.
	Call *Call `json:"call,omitempty"`
	// Builtin is the fully qualified name of a standard library function
	// a call calls.
	Builtin string `json:"builtin,omitempty"`
	// Classifier is set on a reference to a class or object used as a
	// qualifier or value; ImportableName is the fully qualified name an
	// import of what the reference names would have.
	Classifier     bool   `json:"classifier,omitempty"`
	ImportableName string `json:"importableName,omitempty"`
	// Package is set on a name that refers to a package qualifying a
	// reference: the package's fully qualified name, kotlin.math on
	// kotlin.math and kotlin on kotlin in kotlin.math.floor(x).
	Package string `json:"package,omitempty"`
	When    *When  `json:"when,omitempty"`

	FlowNonNull       bool `json:"flowNonNull,omitempty"`
	DefinitelyNonNull bool `json:"definitelyNonNull,omitempty"`
	// NonNullLeftSubtypeRight is set on an elvis expression: whether its
	// left side's non-null type is a subtype of its right side's type.
	NonNullLeftSubtypeRight *bool `json:"nonNullLeftSubtypeRight,omitempty"`
	ImpossibleCast          bool  `json:"impossibleCast,omitempty"`

	// UsedAsExpression is set on an expression whose value is used.
	UsedAsExpression bool `json:"usedAsExpression,omitempty"`
	// Unreachable is set on a statement that can't be reached.
	Unreachable bool `json:"unreachable,omitempty"`
	// SuspendOperation is set on an operation that suspends other than
	// a call: a for loop whose iterator(), hasNext() or next() is a
	// suspend operator. An operator that is written, such as x + y or
	// x++, has a Call.
	SuspendOperation bool `json:"suspendOperation,omitempty"`

	// LambdaParameters are the symbols of a lambda's parameters, its
	// implicit it if ImplicitParameter is set.
	LambdaParameters  []uint32 `json:"lambdaParameters,omitempty"`
	ImplicitParameter bool     `json:"implicitParameter,omitempty"`
	// ReturnTarget is the function or lambda a return leaves;
	// ReceiverTarget the class, function or lambda whose receiver a this
	// is.
	ReturnTarget   *ElementID `json:"returnTarget,omitempty"`
	ReceiverTarget *ElementID `json:"receiverTarget,omitempty"`
	// ImplicitReceiver is set on a call, its callee or a property
	// reference whose receiver is implicit: the lambda with a receiver,
	// extension function, or class or object whose this it is, as
	// Call.ImplicitReceiver for a call.
	ImplicitReceiver *ElementID `json:"implicitReceiver,omitempty"`

	// DelegateType is a delegated property's delegate's type, and
	// DelegatedCalls the getValue and setValue calls the property makes.
	DelegateType   *Type  `json:"delegateType,omitempty"`
	DelegatedCalls []Call `json:"delegatedCalls,omitempty"`

	// Accessors are set on a reference to a property through a receiver,
	// a.p: its getter, and its setter if it has one, as kotlinc's
	// PropertyDescriptor's accessors, whether the reference reads or
	// writes it. A Kotlin property's are named Owner.<get-p> and
	// Owner.<set-p>, a Java one's by their methods, getP and setP. Each
	// lists the accessors it overrides as Targets.
	Accessors []Call `json:"accessors,omitempty"`

	// Deprecated is the deprecation message of what a reference names.
	Deprecated *string `json:"deprecated,omitempty"`
}

// The fact families, as Data.Capabilities lists the computed ones.
const (
	CapTypes             = "types"
	CapSymbols           = "symbols"
	CapCalls             = "calls"
	CapWhen              = "when"
	CapUsedAsExpression  = "used-as-expression"
	CapUnreachable       = "unreachable"
	CapFlowNonNull       = "flow-non-null"
	CapLambdaParameters  = "lambda-parameters"
	CapReturnTargets     = "return-targets"
	CapReceiverTargets   = "receiver-targets"
	CapClassifiers       = "classifiers"
	CapDelegates         = "delegates"
	CapClassMembers      = "class-members"
	CapDeprecation       = "deprecation"
	CapImpossibleCasts   = "impossible-casts"
	CapSubtypeRelations  = "subtype-relations"
	CapSuspendOperations = "suspend-operations"
	CapShadowing         = "shadowing"
	CapCallTargets       = "call-targets"
	CapAnnotations       = "annotations"
	CapImplicitReceivers = "implicit-receivers"
)

// capabilities are the fact families this package computes.
var capabilities = []string{
	CapTypes, CapSymbols, CapCalls, CapWhen, CapUsedAsExpression, CapUnreachable,
	CapFlowNonNull, CapLambdaParameters, CapReturnTargets, CapReceiverTargets,
	CapClassifiers, CapDelegates, CapClassMembers, CapDeprecation, CapAnnotations,
	CapSuspendOperations, CapShadowing, CapCallTargets, CapImpossibleCasts,
}

// Data is all of a Result's facts and symbols, in order: facts by
// snapshot, as passed, and node; symbols by ID.
type Data struct {
	Facts        []Fact   `json:"facts"`
	Symbols      []Symbol `json:"symbols"`
	Capabilities []string `json:"capabilities"`
}

// Result is the outcome of analyzing a module. Its methods return copies
// that callers may change, and are safe for concurrent use.
type Result struct {
	facts   map[ElementID]*Fact
	order   []ElementID
	symbols []Symbol // by ID-1
	diags   []Diagnostic
}

// Fact returns the fact of the node id names.
func (r *Result) Fact(id ElementID) (Fact, bool) {
	if r == nil {
		return Fact{}, false
	}
	f, ok := r.facts[id]
	if !ok {
		return Fact{}, false
	}
	return copyFact(*f), true
}

// Symbol returns the symbol with ID id.
func (r *Result) Symbol(id uint32) (Symbol, bool) {
	if r == nil || id == 0 || int(id) > len(r.symbols) {
		return Symbol{}, false
	}
	return copySymbol(r.symbols[id-1]), true
}

// Diagnostics returns the errors that stopped analysis, if any. Facts
// are only reported without them.
func (r *Result) Diagnostics() []Diagnostic {
	if r == nil {
		return nil
	}
	return slices.Clone(r.diags)
}

// Data returns all facts and symbols.
func (r *Result) Data() Data {
	d := Data{Facts: []Fact{}, Symbols: []Symbol{}, Capabilities: slices.Clone(capabilities)}
	if r == nil {
		return d
	}
	for _, id := range r.order {
		d.Facts = append(d.Facts, copyFact(*r.facts[id]))
	}
	for _, s := range r.symbols {
		d.Symbols = append(d.Symbols, copySymbol(s))
	}
	return d
}

// Capabilities returns the fact families this package computes.
func Capabilities() []string { return slices.Clone(capabilities) }

// Analyze is AnalyzeArena.
func Analyze(ctx context.Context, cfg Config, snapshots []*psi.Snapshot) (*Result, error) {
	return AnalyzeArena(ctx, cfg, snapshots)
}

// AnalyzeArena analyzes snapshots as the sources of one module. It
// returns an error if ctx is done, the configuration is invalid, the
// class path can't be opened, or a snapshot is a script
// (ErrUnsupported). Errors in the sources are the Result's Diagnostics.
func AnalyzeArena(ctx context.Context, cfg Config, snapshots []*psi.Snapshot) (*Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	switch cfg.ExplicitAPIMode {
	case "", "disable", "warning", "strict":
	default:
		return nil, fmt.Errorf("analysis: invalid explicit API mode %q", cfg.ExplicitAPIMode)
	}
	seen := map[*psi.Snapshot]bool{}
	for _, s := range snapshots {
		if s == nil {
			return nil, errors.New("analysis: nil snapshot")
		}
		if seen[s] {
			return nil, fmt.Errorf("analysis: %s passed twice", s.Path())
		}
		seen[s] = true
		if strings.HasSuffix(s.Path(), ".kts") {
			return nil, fmt.Errorf("%w: script %s", ErrUnsupported, s.Path())
		}
	}
	return analyze(ctx, cfg, snapshots)
}

func copyType(t *Type) *Type {
	if t == nil {
		return nil
	}
	c := copyTypeValue(*t)
	return &c
}

func copyTypeValue(t Type) Type {
	t.ParameterOwner = copyID(t.ParameterOwner)
	if t.Arguments != nil {
		args := make([]Type, len(t.Arguments))
		for i, a := range t.Arguments {
			args[i] = copyTypeValue(a)
		}
		t.Arguments = args
	}
	t.Supertypes = slices.Clone(t.Supertypes)
	return t
}

func copyID(id *ElementID) *ElementID {
	if id == nil {
		return nil
	}
	c := *id
	return &c
}

func copyTypes(ts []Type) []Type {
	if ts == nil {
		return nil
	}
	out := make([]Type, len(ts))
	for i, t := range ts {
		out[i] = copyTypeValue(t)
	}
	return out
}

func copyCall(c *Call) *Call {
	if c == nil {
		return nil
	}
	v := copyCallValue(*c)
	return &v
}

func copyCallValue(c Call) Call {
	c.Declaration = copyID(c.Declaration)
	c.DeclaredReceiver = copyType(c.DeclaredReceiver)
	c.Receiver = copyType(c.Receiver)
	c.ImplicitReceiver = copyID(c.ImplicitReceiver)
	c.Parameters = copyTypes(c.Parameters)
	c.ParameterNames = slices.Clone(c.ParameterNames)
	c.Varargs = slices.Clone(c.Varargs)
	c.ArgumentMapping = slices.Clone(c.ArgumentMapping)
	c.ReturnType = copyType(c.ReturnType)
	c.DeclaredReturnType = copyType(c.DeclaredReturnType)
	c.TypeParameterNames = slices.Clone(c.TypeParameterNames)
	c.Annotations = slices.Clone(c.Annotations)
	c.OwnerAnnotations = slices.Clone(c.OwnerAnnotations)
	c.PackageAnnotations = slices.Clone(c.PackageAnnotations)
	c.Crossinline = slices.Clone(c.Crossinline)
	c.Noinline = slices.Clone(c.Noinline)
	c.Targets = copyCalls(c.Targets)
	return c
}

func copyCalls(cs []Call) []Call {
	if cs == nil {
		return nil
	}
	out := make([]Call, len(cs))
	for i, c := range cs {
		out[i] = copyCallValue(c)
	}
	return out
}

func copyFact(f Fact) Fact {
	f.Type = copyType(f.Type)
	f.Call = copyCall(f.Call)
	if f.When != nil {
		w := *f.When
		w.Missing = slices.Clone(w.Missing)
		f.When = &w
	}
	if f.NonNullLeftSubtypeRight != nil {
		b := *f.NonNullLeftSubtypeRight
		f.NonNullLeftSubtypeRight = &b
	}
	f.LambdaParameters = slices.Clone(f.LambdaParameters)
	f.ReturnTarget = copyID(f.ReturnTarget)
	f.ReceiverTarget = copyID(f.ReceiverTarget)
	f.ImplicitReceiver = copyID(f.ImplicitReceiver)
	f.DelegateType = copyType(f.DelegateType)
	f.DelegatedCalls = copyCalls(f.DelegatedCalls)
	f.Accessors = copyCalls(f.Accessors)
	if f.Deprecated != nil {
		s := *f.Deprecated
		f.Deprecated = &s
	}
	return f
}

func copySymbol(s Symbol) Symbol {
	s.Type = copyTypeValue(s.Type)
	s.Declaration = copyID(s.Declaration)
	s.Supertypes = slices.Clone(s.Supertypes)
	return s
}

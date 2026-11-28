package check

import (
	"math"
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/internal/syntax"
	"github.com/barteo/kotlin-compiler-go/internal/types"
)

// value checks an expression whose value is used.
func (c *Checker) value(x syntax.Expr, want types.Type) types.Type {
	t := c.expr(x, want)
	switch t.K {
	case types.Range:
		c.errorf(x.Pos(), "ranges are only supported in 'for' loops and 'in' checks")
		return types.InvalidType
	case types.Unit:
		if want.K != types.Unit {
			t = c.safeUnit(x, t)
		}
	}
	return t
}

// safeUnit returns the type of x, of type t, as a value: a safe call of a
// Unit function, x?.f(), is the Unit object or null, a Unit?.
func (c *Checker) safeUnit(x syntax.Expr, t types.Type) types.Type {
	if u := c.u.ext.class("kotlin/Unit"); t.K == types.Unit && !t.N && u != nil && safeCallOf(x) {
		t = types.ClassType(u).Nullable()
		x.SetType(t)
	}
	return t
}

// returnValue checks x, the value a function returns, expecting want: a
// safe call of a Unit function, x?.f(), is a Unit? there, which kotlinc
// doesn't coerce to a Unit return type: fun f(): Unit = c?.close().
func (c *Checker) returnValue(x syntax.Expr, want types.Type) types.Type {
	t := c.value(x, want)
	if want.K == types.Unit && !want.N {
		t = c.safeUnit(x, t)
	}
	return t
}

// safeUnitCall reports whether x, of type t, is a safe call of a Unit
// function, whose value is a Unit?.
func safeUnitCall(x syntax.Expr, t types.Type) bool {
	return safeCallOf(x) && t.K == types.Class && t.N && t.C.Name == "kotlin/Unit"
}

// rangeExpr checks an expression in a position where ranges are allowed.
func (c *Checker) rangeExpr(x syntax.Expr) types.Type {
	c.allowRange = true
	return c.expr(x, noWant)
}

func (c *Checker) wantBool(x syntax.Expr) {
	if t := x.Type().Flex(); t != types.BoolType && t.K != types.Invalid && t.K != types.Nothing {
		c.errorf(x.Pos(), "type mismatch: inferred type is %s but Boolean was expected", t)
	}
}

func (c *Checker) expr(x syntax.Expr, want types.Type) types.Type {
	if c.file.Annotated != nil {
		c.annotatedExpr(x)
	}
	allowRange, sink := c.allowRange, c.sink
	c.allowRange, c.sink = false, nil
	t := c.expr1(x, want, allowRange, sink)
	if p := c.u.probe; p != nil && c.file == p.File {
		c.probeAt(x, want)
	}
	switch x := x.(type) {
	case *syntax.Ident:
		c.propShadowed(x)
		c.dslProp(x)
		t = c.contextRead(x, t, want)
	case *syntax.Member:
		t = c.contextRead(x, t, want)
	}
	x.SetType(t)
	c.composeRead(x)
	if c.u.lookups != nil {
		c.noteExpr(x, t)
	}
	return t
}

func (c *Checker) expr1(x syntax.Expr, want types.Type, allowRange bool, sink *system) types.Type {
	switch x := x.(type) {
	case *syntax.Ident:
		if x.Object != nil {
			return types.ClassType(x.Object) // an object, already resolved
		}
		if x.Name == "entries" && c.u.ext.kotlin {
			// As in Kotlin, an enum's entries in its own code, and its
			// nested classes', come before the scopes around it, as an
			// outer object's entries or a top-level entries, but not the
			// members of the classes in between, nor a local or a
			// parameter.
			if cls := c.entriesEnum(); cls != nil {
				if sym, _, _, ok := c.lookupVar(x.Name); !ok || sym.Kind != types.Local && sym.Kind != types.Param {
					m := &syntax.Member{X: &syntax.Ident{Name: cls.Name}, Name: x.Name}
					m.P = x.P
					t := c.enumEntries(m, cls)
					m.T = t
					x.Member = m
					return t
				}
			}
		}
		if len(c.recvs) > 0 {
			if v, _, local := c.scope.lookup(x.Name); !local || propParam(v) != nil {
				if t, ok := c.recvMember(x); ok {
					if p := x.Member.Prop; p == nil || !lowPriorityVar(p) || !c.lowPriorityLoses(x.Name) {
						return t
					}
					x.Member = nil
				}
			}
		}
		if c.cls == nil && c.staticCls != nil {
			// In a delegation call's arguments, the class's static scope
			// comes before its companion: C is the nested object C, not
			// the companion's val C.
			if _, _, local := c.scope.lookup(x.Name); !local {
				if cls := c.findClass(x.Name); cls != nil && cls.Outer == c.staticCls && cls.Object && cls != c.staticCls.Companion {
					return objectValue(x, cls)
				}
			}
		}
		sym, t, cast, ok := c.lookupVar(x.Name)
		if ok {
			c.recursiveVarRef(sym, x.P)
		}
		if ok && sym.Kind == types.Global && (c.cls != nil || c.staticCls != nil) {
			// As in Kotlin, the implicit receivers come before the
			// top-level properties: a property of this, an extension
			// property of this, then a member of an object in scope.
			if t, ok := c.receiverProp(x); ok {
				return t
			}
			// So do the static fields the class inherits from Java
			// classes, before imported ones.
			if t, ok := c.inheritedStatic(x); ok {
				return t
			}
		}
		if ok && lowPriorityVar(sym) {
			// An object of the name comes before a property annotated
			// @LowPriorityInOverloadResolution.
			if obj := c.objectNamed(x.Name); obj != nil {
				return objectValue(x, obj)
			}
		}
		if ok && (sym.Ambiguous || c.starVarAmbiguous(x.Name, sym)) {
			c.errorf(x.P, "overload resolution ambiguity: %s", x.Name)
		}
		if ok {
			c.checkTopVar(sym, x.P)
			if sym.Kind == types.Prop && sym.Owner != nil && c.cls != nil && isJavaClass(sym.Owner) && c.memberProp(c.cls, x.Name) == sym {
				c.shadowedField(c.cls, sym, x.P) // a of this
			}
			if v, imported := c.userVars[x.Name]; imported && v == sym {
				c.importAmbiguity(x)
			}
		}
		if ok && sym.Kind == types.Prop && !cast {
			// A property of this, once a smart cast has changed this's
			// type, is the smart-cast type's: in when (this) { is W ->
			// value }, W's value.
			if _, narrowed := c.narrowedThis(); narrowed {
				if mt, ok := c.thisMember(x); ok && !c.sameType(mt, t) {
					return mt
				}
				x.Member = nil
			}
		}
		if !ok {
			if t, ok := c.receiverProp(x); ok {
				return t
			}
			if t, ok := c.inheritedStatic(x); ok {
				return t
			}
			if t, ok := c.importedMember(x); ok {
				return t
			}
			if cls := c.enumInScope(); x.Name == "entries" && cls != nil && c.u.ext.kotlin {
				// An enum's entries in its members and its companion's.
				m := &syntax.Member{X: &syntax.Ident{Name: cls.Name}, Name: x.Name}
				m.P = x.P
				t := c.enumEntries(m, cls)
				m.T = t
				x.Member = m
				return t
			}
			if x.Name == "Unit" || c.unitAlias(x.Name) {
				x.Unit = true
				return types.UnitType
			}
			if obj := c.builtinCompanion(x.Name); obj != nil {
				return objectValue(x, obj) // Int in Int.MAX_VALUE
			}
			cls := c.findClass(x.Name)
			if cls != nil {
				// An object named as a function is the object: a value.
				if obj := objectOf(cls); obj != nil {
					c.checkClassVisible(obj, x.P) // the class, or its private companion
					if !c.usable(obj, x.P) {
						return types.InvalidType // a library's internal companion
					}
					return objectValue(x, obj)
				}
				if obj := c.starObject(x.Name, cls); obj != nil {
					return objectValue(x, obj)
				}
			}
			if len(c.topFuns(x.Name)) > 0 {
				c.errorf(x.P, "function invocation '%s()' expected; use ::%s for a reference", x.Name, x.Name)
			} else if cls != nil {
				c.errorf(x.P, "class '%s' cannot be used as a value", x.Name)
				return types.UnitType // as K2 types it: id(A) infers its T
			} else if rt, ok := c.nullableImplicit(x.Name, nil); ok {
				return c.unsafeImplicitProp(x, rt)
			} else if t, ok := c.inaccessibleOuterProp(x); ok {
				return t
			} else {
				c.errorf(x.P, "unresolved reference: %s", x.Name)
			}
			return types.InvalidType
		}
		if t != sym.Type && sym.Type.IsParam() && want.K != types.Invalid && !want.HasVars() && !c.subtype(t, want) && c.subtype(sym.Type, want) {
			// A smart cast is an intersection with the declared type: x
			// of type T, smart-cast to Boolean or Nothing?, is still a T.
			t, cast = sym.Type, false
		} else if t != sym.Type && want.K == types.Class && !want.HasVars() && !c.subtype(t, want) {
			// As is a smart cast to an interface that the declared type
			// doesn't implement: step, smart-cast to Action, is still a
			// QuestionnaireStep.
			dt := sym.Type
			if !types.MayBeNull(t) {
				dt = dt.NonNull()
			}
			if c.subtype(dt, want) {
				t, cast = dt, false
			}
		}
		t, cast = c.identField(sym, t, cast, x.P)
		x.Sym, x.Cast = sym, cast
		if sym.UnitLocal() {
			x.Unit = true // a local of type Unit has no storage
		}
		if sym.Suspend {
			c.suspendCall(x.P, &types.FunSym{Name: sym.Name, Suspend: true})
		}
		return t
	case *syntax.Checked:
		return x.T
	case *syntax.ClassLit:
		return c.classLit(x, false)
	case *syntax.Try:
		return c.tryExpr(x, want, true)
	case *syntax.Is:
		return c.isExpr(x)
	case *syntax.As:
		return c.asExpr(x)
	case *syntax.Super:
		c.errorf(x.P, "'super' is not an expression, it can only be used on the left-hand side of a dot ('.')")
		return types.InvalidType
	case *syntax.This:
		return c.thisExpr(x)
	case *syntax.IntLit:
		if x.Unsigned {
			return c.unsignedLit(x, want)
		}
		return intLit(x, x.Val, want)
	case *syntax.DoubleLit:
		return types.DoubleType
	case *syntax.FloatLit:
		return types.FloatType
	case *syntax.CharLit:
		return types.CharType
	case *syntax.BoolLit:
		return types.BoolType
	case *syntax.NullLit:
		return types.NullType
	case *syntax.PropRef:
		return c.propRef(x)
	case *syntax.ObjectExpr:
		t := c.objectExpr(x)
		c.scope.narrows = append(c.scope.narrows, c.superArgNarrows(x)...)
		return t
	case *syntax.StringLit:
		for _, p := range x.Parts {
			if p.X != nil {
				c.value(p.X, noWant)
			}
		}
		return types.StringType
	case *syntax.Unary:
		return c.unary(x, want)
	case *syntax.Binary:
		if x.Op != syntax.ELVIS && x.Op != syntax.AND && x.Op != syntax.OR {
			// An elvis's and a condition's operands have the expected
			// type of their own.
			saved := c.operator
			c.operator = x
			defer func() { c.operator = saved }()
		}
		return c.binary(x, want, allowRange)
	case *syntax.Member:
		return c.member(x)
	case *syntax.Call:
		if w := want.Flex().NonNull(); x.Literal && w.K == types.Class && w.C.Array && w.C.Elem.IsPrimitive() {
			// [1, 2] where an IntArray is expected: intArrayOf(1, 2).
			name := strings.TrimPrefix(w.C.KotlinName, "kotlin/")
			x.Fun.(*syntax.Ident).Name = strings.ToLower(name[:1]) + name[1:] + "Of"
		} else if x.Literal && w.K == types.Class && types.IsUnsignedArray(w.C) {
			// [1u, 2u] where a UIntArray is expected: uintArrayOf(1u, 2u).
			name := strings.TrimPrefix(w.C.Name, "kotlin/")
			x.Fun.(*syntax.Ident).Name = strings.ToLower(name[:2]) + name[2:] + "Of"
		}
		t := c.call(x, want, sink)
		c.arrayNothingExpr(x, t)
		return t
	case *syntax.Index:
		return c.index(x, "get", x.Args, want)
	case *syntax.NotNull:
		// As kotlinc's checkNotNull(x: K?): K, x expects the expected
		// type's nullable: findObject(key)!! infers T.
		xw := noWant
		if want.K != types.Invalid && want.K != types.Unit && !want.HasVars() {
			xw = want.Nullable()
		}
		if _, call := x.X.(*syntax.Call); call && sink != nil {
			// And a generic call's variables are the outer call's: x of
			// String by list.getElement(0)!! infers its E.
			n := len(sink.vars)
			c.sink = sink
			t := c.value(x.X, xw)
			c.noteMaybeNull(x, t)
			if len(sink.vars) > n {
				sink.fixups = append(sink.fixups, func(s *system) { x.SetType(s.final(t).NonNull()) })
			}
			return t.NonNull()
		}
		t := c.declaredIfNull(x.X, c.value(x.X, xw))
		c.noteMaybeNull(x, t)
		if t.K == types.Invalid {
			return t
		}
		if k, ok := c.narrowKeyOf(x.X); ok && types.MayBeNull(k.t) {
			// x!! smart-casts x for the code after it.
			k.t = k.t.NonNull()
			c.scope.narrows = append(append(c.scope.narrows, k), c.impliedBy(k)...)
		}
		c.scope.narrows = append(c.scope.narrows, c.notNullCast(x.X)...)
		return t.NonNull()
	case *syntax.Spread:
		// An argument for a vararg parameter: resolve checks where it goes.
		var t types.Type
		if _, call := x.X.(*syntax.Call); call && sink != nil {
			// A generic call's variables are the outer call's, as
			// unspread: T of Arrays.asList(*x.toArray(y)) for a
			// List<String>.
			n := len(sink.vars)
			c.sink = sink
			t = c.value(x.X, want)
			if len(sink.vars) > n {
				sink.fixups = append(sink.fixups, func(s *system) { x.SetType(s.final(t).Flex()) })
			}
		} else {
			t = c.value(x.X, want)
		}
		switch {
		case t.K == types.Invalid:
		case !t.NonNull().IsArray() && !(t.K == types.Class && types.IsUnsignedArray(t.C)):
			c.errorf(x.P, "the spread operator can only be applied to arrays, not to %s", t)
			return types.InvalidType
		case types.MayBeNull(t) && !t.P:
			c.errorf(x.P, "the spread operator cannot be applied to a nullable array: %s", t)
			return types.InvalidType
		}
		return t.Flex()
	case *syntax.If:
		return c.ifExpr(x, want, true)
	case *syntax.When:
		return c.whenExpr(x, want, true)
	case *syntax.Jump:
		c.jump(x)
		return types.NothingType
	case *syntax.Lambda:
		return c.lambda(x, want)
	}
	c.errorf(x.Pos(), "unsupported expression")
	return types.InvalidType
}

// libraryCallable reports whether a simple name can call a function or
// constructor of the class path: a function of an import or a default
// import, or a class.
func (c *Checker) libraryCallable(name string) bool {
	return len(c.kotlinFuns(name, false)) > 0 || c.findClass(name) != nil
}

// lookupVar resolves a name to a variable: locals and parameters, then
// properties of the enclosing class, then top-level properties. It
// returns the variable's type, with smart casts, and whether a smart cast
// changes its class.
// receiverProp checks a name that is no local or property of the class as
// a property of an implicit receiver, in Kotlin's order: one inherited
// from a Java or Kotlin class, a member of this, an extension property of
// this, then a property of an object in scope, such as a companion. An
// extension property of this comes before the companion's members, as
// in kotlinc.
func (c *Checker) receiverProp(x *syntax.Ident) (types.Type, bool) {
	if t, ok := c.inheritedGetter(x); ok {
		return t, true
	}
	if t, ok := c.thisMember(x); ok {
		return t, true
	}
	if t, ok := c.thisExtProp(x); ok {
		return t, true
	}
	if !c.objectHasProp(x.Name) {
		return types.InvalidType, false
	}
	t, ok := c.objectMember(x)
	if ok && x.Member != nil && x.Member.Prop != nil {
		c.companionProtectedProp(x.Member.Prop, x.P)
	}
	return t, ok
}

func (c *Checker) lookupVar(name string) (*types.VarSym, types.Type, bool, bool) {
	if sym, depth := c.scope.find(name); sym != nil {
		switch {
		case sym.Delegate != nil:
			c.capture(sym.Delegate, depth) // reading it calls getValue on that
		case sym.Kind == types.BackingField:
			// field is this's field, or a static one: a lambda captures this.
			if p := sym.Prop; p.Kind != types.Global && !p.StaticField() {
				if v := c.localField(sym); v != nil {
					sym = v
				} else {
					c.captureThis()
				}
			}
		case !sym.UnitLocal():
			c.capture(sym, depth)
		}
		t := c.scope.narrowed(sym)
		if t != sym.Type && isVar(sym) && !c.varStable(sym, t) {
			t = sym.Type // a lambda may assign it
		}
		return sym, t, needsCast(sym.Type, t), true
	}
	if e := c.enumEntryInScope(name); e != nil {
		return e, e.Type, false, true
	}
	if c.cls != nil {
		if p := c.memberProp(c.cls, name); p != nil && (c.propVisible(p) || c.topVar(name) == nil) {
			// A superclass's private property gives way to a top-level
			// one; without one, the use is an error (see checkTopVar).
			c.captureThis()
			ensureVar(p)
			declared := c.memberSubst(c.cls.This(), p.Owner).Apply(p.Type)
			if t := c.scope.narrowed(p); t != p.Type {
				return p, t, needsCast(declared, t), true
			}
			return p, declared, false, true
		}
	}
	if v := c.topVar(name); v != nil {
		ensureVar(v)
		t := c.scope.narrowed(v)
		return v, t, needsCast(v.Type, t), true
	}
	return nil, types.Type{}, false, false
}

// intLit returns the type of an integer literal of value v: Long with an
// L suffix, or the expected type if it is Long, or Byte or Short and v
// fits it; otherwise Int.
func intLit(x *syntax.IntLit, v int64, want types.Type) types.Type {
	if x.Long {
		return types.LongType
	}
	switch want.NonNull() {
	case types.LongType:
		return types.LongType
	case types.ByteType:
		if math.MinInt8 <= v && v <= math.MaxInt8 {
			return types.ByteType
		}
	case types.ShortType:
		if math.MinInt16 <= v && v <= math.MaxInt16 {
			return types.ShortType
		}
	}
	return types.IntType
}

// unsignedLit returns the type of an unsigned literal: UInt, or ULong if
// it doesn't fit or ends in L, or the unsigned type expected if it fits.
func (c *Checker) unsignedLit(x *syntax.IntLit, want types.Type) types.Type {
	name := "kotlin/UInt"
	if x.Long {
		name = "kotlin/ULong"
	}
	v := uint64(x.Val)
	if w := want.NonNull(); types.IsValue(w) && !x.Long {
		switch w.C.Name {
		case "kotlin/ULong":
			name = w.C.Name
		case "kotlin/UByte":
			if v <= math.MaxUint8 {
				name = w.C.Name
			}
		case "kotlin/UShort":
			if v <= math.MaxUint16 {
				name = w.C.Name
			}
		}
	}
	cls := c.u.ext.class(name)
	if cls == nil {
		// As kotlinc, without the standard library's unsigned types.
		c.errorf(x.P, "type of the constant expression cannot be resolved. Please make sure you have the required dependencies for unsigned types in the classpath")
		return types.InvalidType
	}
	if !cls.Value {
		return types.InvalidType
	}
	return types.ClassType(cls)
}

// fitsLit reports whether an integer literal argument can be passed for a
// parameter of type p, once Int doesn't fit: as a Long, Byte or Short,
// or, unsigned and once UInt doesn't, as a ULong, UByte or UShort.
func fitsLit(lit *syntax.IntLit, p types.Type) bool {
	if lit.Unsigned {
		w := p.NonNull()
		if lit.Long || !types.IsValue(w) {
			return false
		}
		switch v := uint64(lit.Val); w.C.Name {
		case "kotlin/ULong":
			return true
		case "kotlin/UByte":
			return v <= math.MaxUint8
		case "kotlin/UShort":
			return v <= math.MaxUint16
		}
		return false
	}
	switch p.NonNull() {
	case types.LongType, types.ByteType, types.ShortType:
		return intLit(lit, lit.Val, p) == p.NonNull()
	}
	return false
}

// unaryExt reports whether prefix operator x on a built-in operand of
// type t, which has no such operator of its own, calls an operator
// extension: +"OK" for a String.unaryPlus().
func (c *Checker) unaryExt(x *syntax.Unary, t types.Type) bool {
	switch n := t.NonNull(); {
	case n.K == types.Invalid || n.K == types.Nothing || n.IsParam():
		return false
	case n.IsNumeric() || n == types.CharType:
		// Int.unaryMinus() and the like, members that win, and Char,
		// whose built-in calls take its extensions' names.
		return false
	case n == types.BoolType && x.Op == syntax.NOT:
		return false
	}
	name := map[syntax.Kind]string{syntax.MINUS: "unaryMinus", syntax.PLUS: "unaryPlus", syntax.NOT: "not"}[x.Op]
	for _, cl := range c.extCallees(name, t) {
		if cl.f.Operator && len(cl.f.Params) == 0 {
			return true
		}
	}
	return false
}

func (c *Checker) unary(x *syntax.Unary, want types.Type) types.Type {
	switch x.Op {
	case syntax.MINUS, syntax.PLUS:
		if lit, ok := x.X.(*syntax.IntLit); ok {
			// A negative literal, which may be a Byte or Short: -128. So
			// may +1.
			if w := want.NonNull(); w == types.ByteType || w == types.ShortType {
				v := lit.Val
				if x.Op == syntax.MINUS {
					v = -v
				}
				t := intLit(lit, v, want)
				lit.SetType(t)
				return t
			}
		}
		t := c.fixedStub(c.value(x.X, want)).Flex()
		x.X, t = builtinBound(x.X, t)
		if !builtinOperand(t) || c.unaryExt(x, t) {
			return c.unaryOp(x, t, want)
		}
		if t.K != types.Invalid && !t.IsNumeric() {
			c.errorf(x.P, "operator '%s' cannot be applied to %s", x.Op, t)
			return types.InvalidType
		}
		if t.K == types.Byte || t.K == types.Short {
			return types.IntType // Byte.unaryMinus() is an Int
		}
		return t
	case syntax.NOT:
		if t := c.fixedStub(c.value(x.X, types.BoolType)); !builtinOperand(t) || c.unaryExt(x, t.Flex()) {
			return c.unaryOp(x, t.Flex(), want)
		}
		c.wantBool(x.X)
		return types.BoolType
	}
	// x++ as a value.
	t := c.incDec(x, true)
	if t.K == types.Invalid {
		return t
	}
	x.T = t
	return t
}

// rangeValue checks a range used as a value, outside for loops and in
// checks, as the standard library's objects that kotlinc creates: a..b is
// IntRange(a, b) or CharRange(a, b), and a..<b, a until b, a downTo b and
// r step n are calls of the infix functions until, downTo and step. The
// call is x.Call; a's type, if checked, is in x.X.
func (c *Checker) rangeValue(x *syntax.Binary, want types.Type) types.Type {
	infix := func(name string, recv syntax.Expr) types.Type {
		m := &syntax.Member{X: recv, Name: name}
		m.P = x.P
		call := &syntax.Call{Fun: m, Args: []syntax.Expr{x.Y}, Infix: true}
		call.P = x.P
		x.Call = call
		return c.expr(call, want)
	}
	switch x.Op {
	case syntax.RANGE:
		// As the bounds' rangeTo: a Long or an Int and a Long give a
		// LongRange, Bytes and Shorts an IntRange.
		xt := x.X.Type().Flex()
		name := "IntRange"
		switch {
		case xt == types.CharType:
			name = "CharRange"
		case xt == types.LongType || isIntLike(xt):
			yt := c.value(x.Y, xt)
			if xt == types.LongType || yt.Flex() == types.LongType {
				name = "LongRange"
				x.X, x.Y = c.convert(x.X, types.LongType), c.convert(checked(x.Y, yt), types.LongType)
			} else {
				x.X, x.Y = c.convert(x.X, types.IntType), c.convert(checked(x.Y, yt), types.IntType)
			}
		}
		cls := c.u.ext.pkgClass("kotlin.ranges", name)
		if cls == nil {
			c.errorf(x.P, "ranges are only supported in 'for' loops and 'in' checks")
			return types.InvalidType
		}
		id := &syntax.Ident{Name: name}
		id.P = x.P
		call := &syntax.Call{Fun: id, Args: []syntax.Expr{x.X, x.Y}}
		call.P = x.P
		x.Call = call
		t := c.construct(call, cls, x.P, want, nil)
		call.SetType(t)
		return t
	case syntax.RANGE_LT, syntax.UNTIL:
		return infix("until", x.X)
	case syntax.DOWN_TO:
		return infix("downTo", x.X)
	}
	// r step n: r as a value, then step.
	rt := c.expr(x.X, noWant)
	return infix("step", checked(x.X, rt))
}

// rangeMember reports whether name is a member of a receiver of type t
// that makes a range, called with n arguments: an integral number's or a
// Char's rangeTo or rangeUntil, intrinsics without a JVM method.
func rangeMember(t types.Type, name string, n int) bool {
	return (name == "rangeTo" || name == "rangeUntil") && n == 1 && (t == types.LongType || isIntLike(t) || t == types.CharType)
}

// memberRange checks a call of rangeTo or rangeUntil on an integral
// number or a Char, r.rangeTo(b), as the range r..b used as a value, and
// r.rangeUntil(b) as r until b: x becomes the call that makes it. It
// reports false, checking only the argument, if that doesn't fit, for
// an extension to take the call.
func (c *Checker) memberRange(x *syntax.Call, fn *syntax.Member, rt types.Type, want types.Type) (types.Type, bool) {
	if len(x.Names) > 0 || x.Trailing() {
		return types.InvalidType, false
	}
	c.noSpread(x)
	at := c.value(x.Args[0], noWant)
	x.Args[0] = checked(x.Args[0], at)
	a := at.Flex()
	if rt == types.CharType && a != types.CharType || rt != types.CharType && a != types.LongType && !isIntLike(a) {
		return types.InvalidType, false
	}
	op := syntax.RANGE
	if fn.Name == "rangeUntil" {
		op = syntax.RANGE_LT
	}
	b := &syntax.Binary{Op: op, X: checked(fn.X, rt), Y: x.Args[0]}
	b.P = x.P
	t := c.rangeValue(b, want)
	if b.Call == nil || t.K == types.Invalid {
		return t, true
	}
	p := x.P
	*x = *b.Call
	x.P = p
	return t, true
}

// nullableIn reports whether x in r, for a range r written in place of
// type rt, tests a nullable x of r's element type, as kotlinc does with
// an inline contains of the standard library: IntRange.contains(Int?).
func (c *Checker) nullableIn(xt, rt types.Type) bool {
	if rt.K != types.Range || !types.MayBeNull(xt) || xt.P {
		return false
	}
	return xt.NonNull() == rt.RangeElem() || xt.NonNull().K == types.Nothing
}

// rangeAsValue turns range r, checked in place for an in check, into the
// programContains reports whether a value of type vt, not elem, is in a
// range of elem by a contains extension of the program's, not the
// standard library's: elem's range class has no such member.
func (c *Checker) programContains(vt, elem types.Type) bool {
	if vt.Flex() == elem || vt.K == types.Invalid {
		return false
	}
	name := map[types.Kind]string{types.Int: "IntRange", types.Long: "LongRange", types.Char: "CharRange"}[elem.K]
	cls := c.u.ext.pkgClass("kotlin.ranges", name)
	if name == "" || cls == nil {
		return false
	}
	for _, cl := range c.extCallees("contains", types.ClassType(cls)) {
		f := cl.f
		if f.Operator && !strings.HasPrefix(f.Facade, "kotlin/") && len(f.Params) == 1 && c.subtype(vt, f.Params[0].Type) {
			return true
		}
	}
	return false
}

// range object it stands for (see rangeValue).
func (c *Checker) rangeAsValue(r *syntax.Binary) syntax.Expr {
	if _, ok := r.Y.(*syntax.Checked); !ok {
		r.Y = checked(r.Y, r.Y.Type())
	}
	t := c.rangeValue(r, noWant)
	r.SetType(t)
	return checked(r, t)
}

// convert returns checked expression x converted to number type t, Int
// or Long, by a call of its toInt or toLong, if it has another type.
func (c *Checker) convert(x syntax.Expr, t types.Type) syntax.Expr {
	if xt := x.Type(); xt.Flex() == t || xt.K == types.Invalid {
		return x
	}
	m := &syntax.Member{X: x, Name: "to" + t.String()}
	m.P = x.Pos()
	call := &syntax.Call{Fun: m}
	call.P = x.Pos()
	return checked(call, c.expr(call, t))
}

// literalArith reports whether x is arithmetic of integer literals
// without a type of their own: 8 * 1024 or -1, or 2.plus(1) or 1.shl(3),
// operator functions called by name, but not 1L or 1u, nor 1.inc().
func literalArith(x syntax.Expr) bool {
	switch x := x.(type) {
	case *syntax.IntLit:
		return !x.Long && !x.Unsigned
	case *syntax.Unary:
		return (x.Op == syntax.MINUS || x.Op == syntax.PLUS) && literalArith(x.X)
	case *syntax.Binary:
		switch x.Op {
		case syntax.PLUS, syntax.MINUS, syntax.STAR, syntax.SLASH, syntax.PERCENT:
			return literalArith(x.X) && literalArith(x.Y)
		}
	case *syntax.Call:
		_, ok := literalOpCall(x)
		return ok
	}
	return false
}

// literalOpCall returns the receiver of x if it is an operator function
// of integer literals called by name, 2.plus(1) or (1 + 2).inv(), which
// is a literal's type, as 2 + 1 is.
func literalOpCall(x *syntax.Call) (*syntax.Member, bool) {
	m, ok := x.Fun.(*syntax.Member)
	if !ok || m.Safe || x.Invoke || len(x.TypeArgs) > 0 || !intLiteralOps[m.Name] || len(x.Args) > 1 || !literalArith(m.X) {
		return nil, false
	}
	unary := m.Name == "inv" || m.Name == "unaryPlus" || m.Name == "unaryMinus"
	if unary != (len(x.Args) == 0) {
		return nil, false
	}
	for _, a := range x.Args {
		if !literalArith(a) {
			return nil, false
		}
	}
	return m, true
}

// isIntOrLong reports whether t is Int or Long, which an Int or Long
// range can contain either of: n in 0 until size.
func isIntOrLong(t types.Type) bool {
	t = t.Flex()
	return t == types.IntType || t == types.LongType
}

// isIntLike reports whether t is an Int, a Short or a Byte, which ranges
// widen to Int.
func isIntLike(t types.Type) bool {
	t = t.Flex()
	return t == types.IntType || t == types.ShortType || t == types.ByteType
}

// integralBound reports whether a range's start of type t makes it one of
// the built-in progressions: an Int, a Long or a Char range. A Double's,
// a Float's or a Comparable's range is a call of its rangeTo, as
// 0.0..1.0 is a ClosedFloatingPointRange.
func integralBound(t types.Type) bool {
	t = t.Flex()
	return isIntLike(t) || t == types.LongType || t == types.CharType || t.K == types.Invalid || t.K == types.Nothing
}

func isRangeOp(k syntax.Kind) bool {
	switch k {
	case syntax.RANGE, syntax.RANGE_LT, syntax.UNTIL, syntax.DOWN_TO, syntax.STEP:
		return true
	}
	return false
}

func (c *Checker) binary(x *syntax.Binary, want types.Type, allowRange bool) types.Type {
	switch x.Op {
	case syntax.AND, syntax.OR:
		c.value(x.X, types.BoolType)
		c.wantBool(x.X)
		whenTrue, whenFalse := c.narrowing(x.X)
		c.push()
		if x.Op == syntax.AND {
			c.scope.narrows = whenTrue
		} else {
			c.scope.narrows = whenFalse
		}
		base := len(c.scope.narrows)
		yt := c.value(x.Y, types.BoolType)
		added := slices.Clone(c.scope.narrows[min(base, len(c.scope.narrows)):])
		c.pop()
		// The right side may not run: what it assigns, as an in-place
		// lambda's r = null in q || r.also { r = null }!!, ends the
		// smart casts that don't hold of its values too.
		n := len(c.scope.narrows)
		c.scope.narrows = append(c.scope.narrows, added...)
		c.mayNotRun(n, nil)
		c.wantBool(x.Y)
		if yt.K == types.Nothing {
			// a && throw E(), a || return: the code after it sees a false,
			// or true.
			if x.Op == syntax.AND {
				c.scope.narrows = append(c.scope.narrows, whenFalse...)
			} else {
				c.scope.narrows = append(c.scope.narrows, whenTrue...)
			}
		}
		return types.BoolType

	case syntax.ELVIS:
		// As kotlinc's elvis(x: K?, y: K): K, the left side expects the
		// expected type's nullable: findObject(key) ?: default infers T.
		xw := noWant
		if want.K != types.Invalid && want.K != types.Unit {
			xw = want.Nullable()
		} else if u := c.u.ext.class("kotlin/Unit"); (want.K == types.Unit || c.unitElvisWant(x)) && u != nil {
			xw = types.ClassType(u).Nullable()
		}
		c.sink = c.takeBranchSink()
		sink, n := c.sink, len(c.errs)
		if call, ok := x.X.(*syntax.Call); ok && x == c.elvisFull && xw.K == types.Invalid && sink == nil && hasLambda(call.Args) {
			if yt, ok := c.elvisRightType(x.Y); ok {
				c.elvisCall, c.elvisWant = call, starsBounded(yt).Nullable()
			}
		}
		xt := c.safeUnit(x.X, c.value(x.X, xw)) // x?.f() ?: y for a Unit f
		c.elvisCall = nil
		if xw.K == types.Invalid && sink == nil && c.cannotInferSince(n) && retryableCall(x.X) {
			// Nothing gives the left call's variables, but K2's K is fixed
			// from the right side first, and the call's result is below it:
			// V of fun <V : A> test(): V? is Nothing in test() ?: error("").
			if yt, ok := c.elvisRightFirst(x.Y); ok {
				c.errs = c.errs[:n]
				xt = c.safeUnit(x.X, c.value(x.X, starsBounded(yt).Nullable()))
			}
		}
		nn, nonNull := len(c.scope.narrows), c.notNullOf(x.X) // the right side's casts may not run
		c.noteMaybeNull(x, xt)
		var yt types.Type
		if lt := xt.Flex().NonNull(); intLitExpr(x.Y) && (lt == types.LongType || lt == types.ShortType || lt == types.ByteType) {
			// An integer literal is of the left side's type: l ?: 0 is
			// a Long for a Long? l, whatever is expected.
			yt = c.value(x.Y, lt)
		} else if want.K == types.Invalid && xt.K != types.Invalid {
			// The right side expects the left's type, unless it can't be
			// one: as kotlinc's K is a supertype of both, ktor's
			// kotlinType?.let { serializer(it) } ?: type.serializer() is a
			// KSerializer<Any?> or a KSerializer<T> for a T : Any.
			// A call of a supertype's class expects that supertype:
			// foo ?: listOf() for a MutableList<String>? foo is a
			// List<String>.
			// A mutable collection's is the read-only one, which the
			// elvis's type is below: values ?: emptyList() for a
			// MutableCollection.
			yw := xt.NonNull()
			if yw.K == types.Class && yw.C.ReadOnly != nil {
				yw.C = yw.C.ReadOnly
			}
			if cls := c.peekClass(x.Y); cls != nil && yw.K == types.Class && yw.C != cls {
				if sup, ok := c.asSuper(yw, cls); ok {
					yw = sup
				}
			}
			// The left's type is a hint, not a type expected of the
			// right side: a lambda's nullable result there makes the
			// elvis nullable, as in a ?: b.let { it.firstOrNull() }.
			saved := c.operator
			c.operator = x
			yt = c.elvisRight(x.Y, yw, xt, true) // see expected.go
			c.operator = saved
		} else {
			yt = c.elvisRight(x.Y, want, xt, false)
		}
		c.mayNotRun(nn, nonNull)
		t, ok := c.join(xt.NonNull(), yt)
		if !ok {
			c.errorf(x.P, "type mismatch: incompatible types %s and %s", xt.NonNull(), yt)
			return types.InvalidType
		}
		if yt.K == types.Unit && !yt.N && t.K == types.Class && t.C.Name == "kotlin/Unit" && !t.N {
			// x?.f() ?: g() for Unit f and g is a Unit, as either call is.
			t = types.UnitType
		}
		if c.u.rec != nil {
			c.u.rec.Elvis(x, c.subtype(xt.NonNull(), yt))
		}
		if yt.K == types.Nothing {
			// x ?: return: the code after it sees x not null.
			c.scope.narrows = append(c.scope.narrows, c.elvisNonNull(x.X)...)
		}
		if yt.P {
			// a ?: javaCall() is of a platform type, as the Java value:
			// null if both are, and unchecked where a T? is expected.
			t = t.Platform()
		}
		if isRaw(t) && javaClass(t.C) {
			// As K2's, whose elvis is a call, the raw type is lost: a
			// raw Generic's ?: return is a Generic<Any!>, whose generic
			// methods aren't erased.
			t = rawLower(t)
		}
		return t

	case syntax.RANGE, syntax.RANGE_LT, syntax.UNTIL, syntax.DOWN_TO, syntax.STEP:
		if xr, ok := x.X.(*syntax.Binary); x.Op == syntax.STEP && c.u.ext.kotlin && (!ok || !isRangeOp(xr.Op) || xr.Op == syntax.STEP) {
			// A progression's step: the standard library's infix step, as
			// in xs.indices step 2.
			m := &syntax.Member{X: checked(x.X, c.value(x.X, noWant)), Name: "step"}
			m.P = x.P
			call := &syntax.Call{Fun: m, Args: []syntax.Expr{x.Y}, Infix: true}
			call.P = x.P
			x.Call = call
			return c.expr(call, want)
		}
		var xt types.Type
		if x.Op != syntax.STEP {
			// a..b on other types than Int and Char: a.rangeTo(b), and the
			// standard library's a until b and a downTo b for UInt.
			xt = c.value(x.X, types.IntType)
			x.X, xt = builtinBound(x.X, xt)
			if xt.Flex().K == types.Nothing {
				// (return) downTo 1u: the other bound's range.
				yt := c.value(x.Y, noWant)
				if x.Y = checked(x.Y, yt); !builtinOperand(yt) {
					xt = yt
				}
			}
			custom := false
			if (x.Op == syntax.RANGE || x.Op == syntax.RANGE_LT) && c.u.ext.kotlin && xt.Flex().K != types.Nothing && integralBound(xt) {
				// 1.."a": a rangeTo of the program's, Int.rangeTo(String).
				if _, ok := x.Y.(*syntax.Checked); !ok {
					w := types.IntType
					if f := xt.Flex(); f == types.LongType || f == types.CharType {
						w = f
					}
					yt := c.value(x.Y, w)
					x.Y, custom = checked(x.Y, yt), !integralBound(yt)
				}
			}
			if !builtinOperand(xt) || (x.Op == syntax.RANGE || x.Op == syntax.RANGE_LT) && (!integralBound(xt) || custom) {
				if x.Op == syntax.UNTIL || x.Op == syntax.DOWN_TO {
					m := &syntax.Member{X: checked(x.X, xt), Name: map[syntax.Kind]string{syntax.UNTIL: "until", syntax.DOWN_TO: "downTo"}[x.Op]}
					m.P = x.P
					call := &syntax.Call{Fun: m, Args: []syntax.Expr{x.Y}, Infix: true}
					call.P = x.P
					x.Call = call
					return c.expr(call, want)
				}
				name := map[syntax.Kind]string{syntax.RANGE: "rangeTo", syntax.RANGE_LT: "rangeUntil"}[x.Op]
				call, t := c.opCall(x.P, checked(x.X, xt), name, []syntax.Expr{x.Y}, want)
				x.Call = call
				return t
			}
			x.X = checked(x.X, xt)
		}
		if !allowRange {
			if c.u.ext.kotlin {
				return c.rangeValue(x, want)
			}
			c.errorf(x.P, "ranges are only supported in 'for' loops and 'in' checks")
		}
		if x.Op == syntax.STEP {
			if xr, ok := x.X.(*syntax.Binary); !ok || !isRangeOp(xr.Op) || xr.Op == syntax.STEP {
				c.errorf(x.P, "'step' must follow a range")
			}
			rt := c.rangeExpr(x.X)
			if rt.K != types.Range && rt.K != types.Invalid && c.u.ext.kotlin {
				// A progression of the standard library's, as 1u..9u:
				// its infix step.
				m := &syntax.Member{X: checked(x.X, rt), Name: "step"}
				m.P = x.P
				call := &syntax.Call{Fun: m, Args: []syntax.Expr{x.Y}, Infix: true}
				call.P = x.P
				x.Call = call
				return c.expr(call, want)
			}
			if rt.RangeElem() == types.LongType {
				c.wantType(x.Y, c.value(x.Y, types.LongType), types.LongType)
			} else {
				c.wantInt(x.Y, c.value(x.Y, types.IntType))
			}
			if rt.K == types.Range {
				return rt
			}
			return types.RangeType
		}
		// An Int, a Long or a Char range. One bound of a Long range may
		// be an Int: 0 until size.
		// Byte and Short bounds give an Int range, as their rangeTo does.
		elem := types.IntType
		switch xt := c.value(x.X, types.IntType).Flex(); {
		case xt == types.CharType, xt == types.LongType:
			elem = xt
		case xt.K == types.Nothing:
			if yt := x.Y.Type().Flex(); yt == types.CharType || yt == types.LongType {
				elem = yt
			}
		case !isIntLike(xt):
			c.wantInt(x.X, xt)
		}
		yt := c.value(x.Y, elem)
		if ch, ok := x.Y.(*syntax.Checked); ok && ch.SAM == nil && ch.T == ch.X.Type() {
			x.Y = ch.X // checked above, and still a constant: 1..100
		}
		switch {
		case yt.Flex().K == types.Nothing:
		case elem == types.IntType && yt.Flex() == types.LongType:
			elem = types.LongType
		case elem == types.LongType && isIntLike(yt), elem == types.IntType && isIntLike(yt):
		default:
			c.wantType(x.Y, yt, elem)
		}
		return types.RangeOf(elem)

	case syntax.IN, syntax.NOT_IN:
		if r, ok := x.Y.(*syntax.Binary); !ok || !isRangeOp(r.Op) {
			return c.containsOp(x)
		} else if r.Op == syntax.STEP {
			c.errorf(x.Y.Pos(), "'in' is only supported with ranges written in place, without 'step'")
		}
		rt := c.rangeExpr(x.Y)
		if rt.K != types.Range && rt.K != types.Invalid {
			// A range of another type, from rangeTo: y.contains(x).
			x.Y = checked(x.Y, rt)
			return c.containsOp(x)
		}
		elem := rt.RangeElem()
		vt := c.value(x.X, elem)
		x.X, vt = builtinBound(x.X, vt)
		if c.programContains(vt, elem) {
			// The program's contains, as ClosedRange<Int>.contains(Long),
			// comes before the standard library's.
			x.X, x.Y = checked(x.X, vt), c.rangeAsValue(x.Y.(*syntax.Binary))
			return c.containsOp(x)
		}
		if c.nullableIn(vt, rt) {
			// The standard library's inline IntRange.contains(Int?).
			x.X, x.Y = checked(x.X, vt), c.rangeAsValue(x.Y.(*syntax.Binary))
			return c.containsOp(x)
		}
		if !((isIntLike(vt) || vt.Flex() == types.LongType) && isIntOrLong(elem)) {
			if f := vt.Flex(); f != elem && f.K != types.Invalid && c.u.ext.kotlin {
				// Another contains of the range's: "s" in 0..1 with
				// IntRange.contains(String).
				x.X, x.Y = checked(x.X, vt), c.rangeAsValue(x.Y.(*syntax.Binary))
				return c.containsOp(x)
			}
			c.wantType(x.X, vt, elem)
		}
		return types.BoolType

	case syntax.EQ, syntax.NE, syntax.EQ_EQ_EQ, syntax.NOT_EQ_EQ:
		xt := c.value(x.X, noWant)
		yWant := xt
		if l, ok := x.Y.(*syntax.Lambda); ok && l.Ref != nil {
			// As in Kotlin, a reference compared is checked on its own,
			// as equals' argument: ::top == Int::intExt.
			yWant = noWant
		}
		if signedIntLit(x.Y) && !c.plugin {
			// An integer literal compared is an Int, as on the left: x ==
			// 3 for a Long x is an error, as 3 == x is. A plugin's code
			// compares a property with its default value, a literal of
			// the property's type.
			yWant = noWant
		}
		yt := c.value(x.Y, yWant)
		x.OpType = c.castEquality(x, xt, yt)
		c.noteIdentTypes(x, xt, yt)
		return types.BoolType
	}

	opWant := noWant
	if want.NonNull() == types.LongType && literalArith(x) {
		// Arithmetic of integer literals is a Long where one is
		// expected: val n: Long = 8 * 1024.
		opWant = types.LongType
	}
	// A builder's element is its type, which may be a built-in one:
	// get(0) + "y" in buildList { add("x") } (see fixedStub).
	xt := c.fixedStub(c.value(x.X, opWant)).Flex()
	x.X, xt = builtinBound(x.X, xt)
	if !builtinOperand(xt) {
		switch x.Op {
		case syntax.PLUS, syntax.MINUS, syntax.STAR, syntax.SLASH, syntax.PERCENT,
			syntax.LT, syntax.GT, syntax.LE, syntax.GE:
			return c.binaryOp(x, xt, want)
		}
	}
	if name := binaryOpName(x.Op); name != "" && nullableOperand(xt) && c.opExtension(xt, name, x.Y) {
		// An operator extension of a nullable receiver: Int?.plus(Int),
		// Long?.compareTo(Long?).
		return c.binaryOp(x, xt, want)
	}
	if _, lambda := x.Y.(*syntax.Lambda); lambda && binaryOpName(x.Op) != "" {
		// A built-in operator takes no lambda, an operator function may,
		// which gives it its type: 3 * { verify() } for an
		// Int.times(block: Scope.() -> Unit).
		return c.binaryOp(x, xt, want)
	}
	yWant := xt
	switch x.Y.(type) {
	case *syntax.If, *syntax.When, *syntax.Try:
		// An operand's branches have their own types: n - (if (s) 0 else
		// pos) subtracts an Int from a Long.
		yWant = noWant
	}
	yt := c.fixedStub(c.value(x.Y, yWant)).Flex() // 1 + get(0), as get(0) + 1
	x.Y, yt = builtinBound(x.Y, yt)
	if xt.K == types.Invalid || yt.K == types.Invalid {
		return types.InvalidType
	}
	switch x.Op {
	case syntax.PLUS:
		if xt.Flex() == types.StringType.Nullable() && c.opExtension(xt, "plus", x.Y) {
			return c.binaryOp(x, xt, want) // a String?.plus extension
		}
		if xt.NonNull() == types.StringType || xt.K == types.Null {
			// null + x is the built-in String?.plus(Any?), as s + x for
			// a String? s: "null" and x's string.
			x.OpType = types.StringType
			return types.StringType
		}
		if xt == types.CharType && yt == types.StringType {
			// Char.plus(String), the stdlib's inline-only operator.
			x.OpType = types.StringType
			return types.StringType
		}
		fallthrough
	case syntax.MINUS, syntax.STAR, syntax.SLASH, syntax.PERCENT:
		op, t, ok := arith(x.Op, xt, yt)
		if !ok && (!builtinOperand(yt) || c.opExtension(xt, opNames[x.Op], checked(x.Y, yt))) {
			// An operator extension of a built-in type: 3 * v is
			// 3.times(v).
			x.Y = checked(x.Y, yt)
			return c.binaryOp(x, xt, want)
		}
		if !ok && (never(xt) || never(yt)) {
			// An operand that never completes, as todo() * "": the
			// operator is dead code.
			x.OpType = types.NothingType
			return types.NothingType
		}
		if !ok {
			c.errorf(x.P, "operator '%s' cannot be applied to %s and %s", x.Op, xt, yt)
			return types.InvalidType
		}
		x.OpType = op
		return t
	case syntax.LT, syntax.GT, syntax.LE, syntax.GE:
		switch {
		case xt.IsNumeric() && yt.IsNumeric():
			x.OpType = types.Wider(xt, yt)
		case xt == types.CharType && yt == types.CharType:
			x.OpType = types.CharType
		case xt == types.StringType && yt == types.StringType:
			x.OpType = types.StringType
		case c.comparable(xt, yt):
			x.OpType = types.ClassType(c.u.ext.class("java/lang/Comparable"))
		case xt.IsParam(), c.opExtension(xt, "compareTo", checked(x.Y, yt)):
			// An operator extension of a type parameter: a context
			// parameter's T.compareTo, as Arrow's x <= y in
			// with(comparator). Or of a built-in type, for another type
			// than its own compareTo takes: Int.compareTo(Char).
			x.Y = checked(x.Y, yt)
			return c.binaryOp(x, xt, want)
		default:
			c.errorf(x.P, "operator '%s' cannot be applied to %s and %s", x.Op, xt, yt)
		}
		return types.BoolType
	}
	c.errorf(x.P, "unsupported operator %s", x.Op)
	return types.InvalidType
}

// arith returns the type an arithmetic operator computes in, op, and its
// result's type. Numbers are computed in the wider of their types, with
// Byte and Short promoted to Int. A Char plus or minus an Int is a Char,
// and the difference of two Chars an Int; both are computed as Ints.
func arith(k syntax.Kind, xt, yt types.Type) (op, result types.Type, ok bool) {
	switch {
	case xt.IsNumeric() && yt.IsNumeric():
		w := types.Wider(xt, yt)
		return w, w, true
	case xt != types.CharType:
	case (k == syntax.PLUS || k == syntax.MINUS) && yt == types.IntType:
		return types.IntType, types.CharType, true
	case k == syntax.MINUS && yt == types.CharType:
		return types.IntType, types.IntType, true
	}
	return types.InvalidType, types.InvalidType, false
}

// comparable reports whether x < y compiles to x.compareTo(y): x is a
// non-null Comparable<T> and y a T.
func (c *Checker) comparable(xt, yt types.Type) bool {
	cmp := c.u.ext.class("java/lang/Comparable")
	if cmp == nil || types.MayBeNull(xt) {
		return false
	}
	sup, ok := c.asSuper(xt, cmp)
	if !ok || sup.A.Len() != 1 || sup.A.Get(0).V == types.Star || sup.A.Get(0).V == types.Out {
		return false
	}
	return c.subtype(yt, sup.A.Get(0).T)
}

func (c *Checker) wantInt(x syntax.Expr, t types.Type) { c.wantType(x, t, types.IntType) }

// wantType checks that x, of type t, has the builtin type w.
func (c *Checker) wantType(x syntax.Expr, t, w types.Type) {
	if t = t.Flex(); t != w && t.K != types.Invalid && w.K != types.Invalid {
		c.errorf(x.Pos(), "type mismatch: inferred type is %s but %s was expected", t, w)
	}
}

// equality checks the operands of ==, or of === (op), and returns the
// comparison type: a non-null primitive for primitive comparison, otherwise
// a reference type. The operands of === are compatible when those of ==
// are; a primitive compared with a reference is boxed, as with kotlinc.
func (c *Checker) equality(p syntax.Pos, op syntax.Kind, xt, yt types.Type) types.Type {
	if xt.K == types.Invalid || yt.K == types.Invalid {
		return types.InvalidType
	}
	if xt.K == types.Unit || yt.K == types.Unit {
		// kotlin.Unit is an object: x == Unit for an x of type R or Any
		// calls equals with Unit.INSTANCE.
		o := yt
		if xt.K != types.Unit {
			o = xt
		}
		if o.K == types.Unit || o.K == types.Null || o.IsParam() || o.NonNull().K == types.Any {
			return types.AnyType.Nullable()
		}
		c.errorf(p, "operator '==' cannot be applied to %s and %s", xt, yt)
		return types.InvalidType
	}
	if xt.K == types.Null || yt.K == types.Null {
		return types.NullType
	}
	if op == syntax.EQ || op == syntax.NE {
		if f := floatOf(xt); f.K != types.Invalid && f == floatOf(yt) {
			// As in kotlinc, floating-point numbers, nullable or not, are
			// equal as IEEE 754 says: 0.0 == -0.0 and NaN != NaN.
			if types.MayBeNull(xt) || types.MayBeNull(yt) {
				return f.Nullable()
			}
			return f
		}
	}
	if (op == syntax.EQ_EQ_EQ || op == syntax.NOT_EQ_EQ) && (types.IsValue(xt) || types.IsValue(yt)) {
		c.errorf(p, "identity equality for arguments of types '%s' and '%s' is prohibited", xt, yt)
		return types.InvalidType
	}
	if xt.IsParam() || yt.IsParam() || xt.K == types.Class && yt.K == types.Class && xt.C == yt.C {
		// A type parameter can stand for any type its bounds allow, and
		// instances of one class with different type arguments may still
		// be equal.
		if c.paramEqualityInapplicable(op, xt, yt) {
			name := "=="
			if op == syntax.EQ_EQ_EQ || op == syntax.NOT_EQ_EQ {
				name = "==="
			}
			c.errorf(p, "operator '%s' cannot be applied to %s and %s", name, xt, yt)
			return types.InvalidType
		}
		return types.AnyType.Nullable()
	}
	if xt.NonNull() != yt.NonNull() {
		// Values of related reference types, such as Any and String, are
		// compared with equals().
		x, y := xt.NonNull(), yt.NonNull()
		if c.subtype(x, y) && !y.IsPrimitive() || c.subtype(y, x) && !x.IsPrimitive() || c.mayBeSame(x, y) {
			return types.AnyType.Nullable()
		}
		if !c.equalityInapplicable(op, xt, yt) {
			return types.AnyType.Nullable()
		}
		name := "=="
		if op == syntax.EQ_EQ_EQ || op == syntax.NOT_EQ_EQ {
			name = "==="
		}
		c.errorf(p, "operator '%s' cannot be applied to %s and %s", name, xt, yt)
		return types.InvalidType
	}
	if xt.IsPrimitive() && yt.IsPrimitive() {
		return xt
	}
	return xt.Nullable()
}

// equalityInapplicable reports whether K2 rejects comparing values of
// unrelated types xt and yt with op, as FirEqualityCompatibilityChecker
// does. == is only checked where a primitive type, String, an enum or a
// value class is involved, === where one of them isn't nullable; either
// only if one class is final or both are classes, not interfaces. Where
// both types can have subtypes, as K1 had it, K2 only warns: A == B for
// an interface A and a class B is allowed, and so is a === b for an open
// class and an abstract one, and a == b for two nullable enums.
func (c *Checker) equalityInapplicable(op syntax.Kind, xt, yt types.Type) bool {
	return c.equalityCompat(op, xt, yt) == eqError
}

// The results of equalityCompat.
const (
	eqOK = iota
	eqWarning
	eqError
)

// equalityCompat is equalityInapplicable with K2's warning:
// EQUALITY_NOT_APPLICABLE_WARNING where it is inapplicable but K1 let
// both types have subtypes.
func (c *Checker) equalityCompat(op syntax.Kind, xt, yt types.Type) int {
	x, y := xt.Flex().NonNull(), yt.Flex().NonNull()
	if op == syntax.EQ || op == syntax.NE {
		if (c.enumType(x) || c.enumType(y)) && types.MayBeNull(xt) && types.MayBeNull(yt) {
			// K1 let a nullable enum be compared with any nullable type,
			// as both may be null: K2 only warns (see
			// enumComparisonWarning).
			return eqOK
		}
		if !c.builtinForEquality(x) && !c.builtinForEquality(y) && !types.IsValue(x) && !types.IsValue(y) {
			return eqOK
		}
	} else if !types.IsPrimitiveKind(x.K) && !types.IsPrimitiveKind(y.K) && types.MayBeNull(xt) && types.MayBeNull(yt) {
		// A primitive has no identity, nullable or not: a === b for an
		// Int? and a String? is an error.
		return eqOK
	}
	final := func(t types.Type) bool {
		switch t.K {
		case types.Class:
			t.C.Ensure()
			return !t.C.Interface && t.C.Final
		case types.Any:
			return false
		}
		return t.K != types.TypeParam
	}
	class := func(t types.Type) bool {
		if t.K == types.Class {
			return !t.C.Interface
		}
		return t.K != types.TypeParam
	}
	if !final(x) && !final(y) && !(class(x) && class(y)) {
		return eqOK
	}
	if canHaveSubtypesK1(x) && canHaveSubtypesK1(y) && c.missedByK1Enums(xt, yt) {
		return eqWarning
	}
	return eqError
}

// paramEqualityInapplicable reports whether K2 rejects comparing a value
// of type parameter type with a value of a type that isn't one, as it
// compares the intersection of the parameter's bounds: comparing with a
// type that one bound rules out, such as an Int with a P : Buffered, is
// an error. An unbounded parameter, or two parameters, can be compared,
// and an enum with a parameter only draws a warning.
func (c *Checker) paramEqualityInapplicable(op syntax.Kind, xt, yt types.Type) bool {
	pt, ot := xt, yt
	if pt.K != types.TypeParam {
		pt, ot = yt, xt
	}
	if pt.K != types.TypeParam || ot.IsParam() || ot.Flex().NonNull().K == types.TypeParam {
		return false
	}
	if c.enumType(ot.Flex().NonNull()) {
		// K1 only checked enums against enums: K2 warns (see
		// enumComparisonWarning).
		return false
	}
	for _, b := range pt.Param().Bounds {
		if b.IsParam() || b.K == types.Invalid {
			continue
		}
		if types.MayBeNull(pt) {
			b = b.Nullable()
		}
		x, y := b.Flex().NonNull(), ot.Flex().NonNull()
		if x == y || c.subtype(x, y) && !y.IsPrimitive() || c.subtype(y, x) && !x.IsPrimitive() || c.mayBeSame(x, y) {
			continue
		}
		if c.equalityInapplicable(op, b, ot) {
			return true
		}
	}
	return false
}

// builtinForEquality reports whether t is a primitive type, String or an
// enum, whose comparisons K2 checks.
func (c *Checker) builtinForEquality(t types.Type) bool {
	if types.IsPrimitiveKind(t.K) || t.K == types.String {
		return true
	}
	return c.enumType(t)
}

// enumType reports whether t is of an enum class.
func (c *Checker) enumType(t types.Type) bool {
	if t.K != types.Class {
		return false
	}
	t.C.Ensure()
	return t.C.Super != nil && t.C.Super.Name == "java/lang/Enum"
}

// missedByK1Enums reports whether K1's check of enum comparisons let xt and
// yt be compared (isIncompatibleEnums), which K2 then only warns about.
func (c *Checker) missedByK1Enums(xt, yt types.Type) bool {
	switch {
	case !c.enumType(xt.Flex().NonNull()) && !c.enumType(yt.Flex().NonNull()):
		return true
	case types.MayBeNull(xt) && types.MayBeNull(yt):
		return true
	case xt.K == types.Nothing || yt.K == types.Nothing:
		return true
	}
	return false
}

// canHaveSubtypesK1 reports whether a non-null type t can have subtypes,
// as K1's TypeUtils.canHaveSubtypes has it: a class that isn't final or
// is an enum can,
// and so can a final generic one with a star, an in projection or an out
// argument that can.
func canHaveSubtypesK1(t types.Type) bool {
	switch t.K {
	case types.Any, types.TypeParam:
		return true
	case types.Class:
	default:
		return false
	}
	t.C.Ensure()
	if t.C.Interface || !t.C.Final || t.C.Super != nil && t.C.Super.Name == "java/lang/Enum" {
		// K1's isFinalClass is false of an enum class, whose entries
		// may be its subclasses.
		return true
	}
	if t.A == nil {
		return false
	}
	for i, a := range t.A.List {
		v := a.V
		if v == types.Inv && i < len(t.C.TypeParams) {
			v = t.C.TypeParams[i].Var
		}
		switch v {
		case types.Star, types.In:
			return true
		case types.Out:
			if types.MayBeNull(a.T) || canHaveSubtypesK1(a.T) {
				return true
			}
		}
	}
	return false
}

// componentEquality reports whether op applies to a component of
// intersection xt or yt and the other operand's type, or a component of
// it.
func (c *Checker) componentEquality(p syntax.Pos, op syntax.Kind, xt, yt types.Type) bool {
	if !xt.IsIntersection() && !yt.IsIntersection() {
		return false
	}
	comps := func(t types.Type) []types.Type {
		if t.IsIntersection() {
			return t.Param().Bounds
		}
		return []types.Type{t}
	}
	n := len(c.errs)
	defer func() { c.errs = c.errs[:n] }()
	for _, a := range comps(xt) {
		for _, b := range comps(yt) {
			if a.IsIntersection() || b.IsIntersection() {
				continue
			}
			if t := c.equality(p, op, a, b); t.K != types.Invalid && len(c.errs) == n {
				return true
			}
			c.errs = c.errs[:n]
		}
	}
	return false
}

// castEquality is equality for the operands of x, of types xt and yt,
// where an operand smart-cast to another class is compared as Kotlin
// does: two numbers, as numbers of the wider type, -0.0 == 0 for x is
// Double and y is Int, and other operands whose declared types may be
// compared as values of those types, with equals.
func (c *Checker) castEquality(x *syntax.Binary, xt, yt types.Type) types.Type {
	dx, cx := declaredOf(x.X)
	dy, cy := declaredOf(x.Y)
	if (x.Op == syntax.EQ || x.Op == syntax.NE) && c.mayBeNullEnums(originalType(x.X, xt), originalType(x.Y, yt)) {
		// Two nullable enums, as declared, even if smart cast not to be
		// null: kotlinc only warns (see enumComparisonWarning).
		return types.AnyType.Nullable()
	}
	if !cx && !cy || xt.K == types.Invalid || yt.K == types.Invalid {
		return c.equality(x.P, x.Op, xt, yt)
	}
	if !cx {
		dx = xt
	}
	if !cy {
		dy = yt
	}
	n := len(c.errs)
	if xn, yn := xt.Flex().NonNull(), yt.Flex().NonNull(); xn.IsNumeric() && yn.IsNumeric() && xn != yn {
		if t := c.equality(x.P, syntax.EQ, dx, dy); t.K != types.Invalid {
			if x.Op == syntax.EQ || x.Op == syntax.NE {
				// Inapplicable to the smart-cast types: kotlinc warns.
				c.warnf(syntax.StartPos(x), "EQUALITY_NOT_APPLICABLE_WARNING", "operator '%s' cannot be applied to '%s' and '%s'", x.Op, xt, yt)
			}
			t = types.Wider(xn, yn)
			if types.MayBeNull(xt) || types.MayBeNull(yt) {
				t = t.Nullable()
			}
			return t
		}
		c.errs = c.errs[:n]
	}
	if t := c.equality(x.P, x.Op, xt, yt); t.K != types.Invalid {
		return t
	}
	m := len(c.errs)
	if t := c.equality(x.P, x.Op, dx, dy); t.K != types.Invalid {
		// Compared as the declared types: the operands are read as those.
		// As kotlinc, a comparison inapplicable only to the smart-cast
		// types is EQUALITY_NOT_APPLICABLE_WARNING, but of enums or of
		// values without identity.
		general := m == n+1 && strings.HasPrefix(c.errs[n].Msg, "operator '") && c.generalEquality(x.Op, xt, yt)
		c.errs = c.errs[:n]
		if general && c.componentEquality(x.P, x.Op, xt, yt) {
			// As K2, an intersection compares as any of its components:
			// x == "" for an x smart cast to String & Int.
			general = false
		}
		if general {
			c.warnf(syntax.StartPos(x), "EQUALITY_NOT_APPLICABLE_WARNING", "operator '%s' cannot be applied to '%s' and '%s'", x.Op, xt, yt)
		}
		for _, e := range []syntax.Expr{x.X, x.Y} {
			if id, ok := e.(*syntax.Ident); ok && id.Cast {
				id.Cast = false
				id.SetType(id.Sym.Type)
			}
		}
		return t
	}
	c.errs = c.errs[:m]
	return types.InvalidType
}

// generalEquality reports whether kotlinc's diagnostic of comparing
// values of types xt and yt with op, if it is inapplicable, is
// EQUALITY_NOT_APPLICABLE: not of enums, nor an identity comparison of a
// value without identity.
func (c *Checker) generalEquality(op syntax.Kind, xt, yt types.Type) bool {
	x, y := xt.Flex().NonNull(), yt.Flex().NonNull()
	if c.enumType(x) || c.enumType(y) {
		return false
	}
	if op == syntax.EQ_EQ_EQ || op == syntax.NOT_EQ_EQ {
		return !x.IsPrimitive() && !y.IsPrimitive() && !types.IsValue(x) && !types.IsValue(y)
	}
	return true
}

// originalType returns the type of x, of type t, before smart casts: a
// variable's declared type.
func originalType(x syntax.Expr, t types.Type) types.Type {
	if id, ok := x.(*syntax.Ident); ok && id.Sym != nil && id.Sym.Kind != types.Static && id.Sym.Type.K != types.Invalid {
		return id.Sym.Type
	}
	return t
}

// mayBeNullEnums reports whether values of types xt and yt are both of
// enums, or null.
func (c *Checker) mayBeNullEnums(xt, yt types.Type) bool {
	return types.MayBeNull(xt) && types.MayBeNull(yt) && c.enumType(xt.Flex().NonNull()) && c.enumType(yt.Flex().NonNull())
}

// declaredOf returns the declared type of x if x is a variable that a
// smart cast gives another class.
func declaredOf(x syntax.Expr) (types.Type, bool) {
	if id, ok := x.(*syntax.Ident); ok && id.Cast && id.Sym != nil && id.Sym.Type.K != types.Invalid {
		return id.Sym.Type, true
	}
	return types.Type{}, false
}

// floatOf returns Double or Float if values of type t are of that type:
// t is it, nullable or not, or a type parameter bounded by it, or else
// InvalidType.
func floatOf(t types.Type) types.Type {
	b := t.Flex().NonNull()
	for b.K == types.TypeParam {
		b = b.Param().Bound().Flex().NonNull()
	}
	if b.K == types.Double || b.K == types.Float {
		return b
	}
	return types.InvalidType
}

// mayBeSame reports whether values of class types x and y may be one
// object although neither type is a subtype of the other: their classes
// are, with other type arguments, or one is an interface and the other
// class can be subclassed. ktor compares a Continuation<TSubject> with an
// object that is a Continuation<Unit>.
func (c *Checker) mayBeSame(x, y types.Type) bool {
	if x.K != types.Class && y.K == types.Class {
		x, y = y, x
	}
	if x.K != types.Class {
		return false
	}
	if y.K != types.Class {
		// A built-in type and a supertype of its class with other type
		// arguments: Double and Comparable<Float>.
		_, ok := c.asSuper(y, x.C)
		return ok
	}
	if _, ok := c.asSuper(x, y.C); ok {
		return true
	}
	if _, ok := c.asSuper(y, x.C); ok {
		return true
	}
	x.C.Ensure()
	y.C.Ensure()
	if x.C.Annotation || y.C.Annotation {
		// As with kotlinc, an annotation may be compared with any
		// object: A == B for two annotation classes.
		return true
	}
	return x.C.Interface && !y.C.Final || y.C.Interface && !x.C.Final
}

// declaredIfNull returns the type of receiver x, of type t: a variable
// that a null check smart-casts to Nothing? has its declared type's
// members too, as Kotlin's intersection type has, s?.length for a
// String? s known to be null, so x is seen with its declared type.
//
// So has a variable of a class type smart-cast to a type parameter without
// bounds: ktor's first.remove() for a first: Node known to be a T.
func (c *Checker) declaredIfNull(x syntax.Expr, t types.Type) types.Type {
	id, ok := x.(*syntax.Ident)
	if !ok || id.Sym == nil || id.Sym.Type.K == types.Null || id.Sym.Type.K == types.Invalid {
		return t
	}
	declared := id.Sym.Type
	switch {
	case t.K == types.Null:
		// A platform value known to be null is a Nothing?, whose members
		// need a safe call: my.foo() after my == null.
		declared = declared.Nullable()
	case t.IsParam() && len(t.Param().Bounds) == 0 && !declared.IsParam() && declared.K != types.Any:
		if !types.MayBeNull(t) {
			declared = declared.NonNull()
		}
	default:
		return t
	}
	id.Cast = false
	id.SetType(declared)
	return declared
}

// castView returns the type of receiver x, of type t, that its member
// name is looked up in. A variable smart-cast to a type unrelated to its
// declared type has the members of both, as Kotlin's intersection type
// has: frame.realCaller() for a frame: CoroutineStackFrame after frame as
// Continuation<*>. If t has no member or extension name and the declared
// type has, x is seen with its declared type.
func (c *Checker) castView(x syntax.Expr, t types.Type, name string) types.Type {
	id, ok := x.(*syntax.Ident)
	if !ok || id.Sym == nil || !id.Cast {
		return t
	}
	declared := id.Sym.Type
	if types.MayBeNull(declared) && !types.MayBeNull(t) {
		declared = declared.NonNull()
	}
	if t.NonNull().K != types.Class || declared.NonNull().K != types.Class || c.subtype(t, declared) ||
		c.hasNamed(t, name) || !c.hasNamed(declared, name) {
		return t
	}
	id.Cast = false
	id.SetType(declared)
	return declared
}

// hasNamed reports whether a value of type t has a member or an extension
// function or property called name.
func (c *Checker) hasNamed(t types.Type, name string) bool {
	if cls := c.classOf(t.NonNull()); cls != nil {
		cls.Ensure()
		if len(c.javaMethods(cls, name)) > 0 || c.hasProp(cls, name) {
			return true
		}
	}
	if len(c.extCallees(name, t)) > 0 {
		return true
	}
	n := len(c.errs)
	_, found := c.extProp(&syntax.Member{Name: name}, t)
	c.errs = c.errs[:n]
	return found
}

// lowPriorityVar reports whether property v of the program is annotated
// @LowPriorityInOverloadResolution, which a property or object of the
// same name in another scope wins over.
func lowPriorityVar(v *types.VarSym) bool {
	d, ok := v.Decl.(*syntax.PropDecl)
	return ok && lowPriorityMods(&d.Modifiers)
}

// lowPriorityLoses reports whether a low-priority member property name
// gives way to a top-level property that isn't low-priority, or to an
// object: in fun Foo.test(), bar is a top-level bar, not Foo's
// @LowPriorityInOverloadResolution one.
func (c *Checker) lowPriorityLoses(name string) bool {
	if c.objectNamed(name) != nil {
		return true
	}
	n := len(c.errs)
	sym, _, _, ok := c.lookupVar(name)
	c.errs = c.errs[:n]
	return ok && sym.Kind == types.Global && !lowPriorityVar(sym)
}

// objectNamed returns the object that name refers to as a value, if any.
func (c *Checker) objectNamed(name string) *types.ClassSym {
	if _, _, local := c.scope.lookup(name); local {
		return nil
	}
	cls := c.findClass(name)
	if cls == nil {
		return nil
	}
	if obj := objectOf(cls); obj != nil {
		return obj
	}
	return c.starObject(name, cls)
}

// suspendRecv returns the suspend function type that receiver fn.X, a
// callable reference of type t, is converted to for a call of fn.Name:
// one that names no member or extension of t but an extension of the
// suspend type.
func (c *Checker) suspendRecv(fn *syntax.Member, t types.Type) (types.Type, bool) {
	l, ok := fn.X.(*syntax.Lambda)
	if !ok || l.Ref == nil || t.N || t.K != types.Class || t.A == nil || c.extCallees(fn.Name, t) != nil || c.hasNamed(t, fn.Name) {
		return types.InvalidType, false
	}
	n, suspend, ok := kfunctionArity(t.C)
	if !ok || suspend {
		return types.InvalidType, false
	}
	ft, ok := c.asSuper(t, c.u.ext.funcs.Class(n))
	if !ok || ft.E || ft.X != 0 {
		return types.InvalidType, false
	}
	ps := make([]types.Type, n)
	for i := range ps {
		ps[i] = ft.A.List[i].T
	}
	st := c.u.ext.funcs.SuspendType(ps, ft.A.List[n].T)
	return st, len(c.extCallees(fn.Name, st)) > 0
}

// builtinNamed reports whether builtin type t, as Int or String, has a
// member or extension name.
func (c *Checker) builtinNamed(t types.Type, name string) bool {
	for n := range 4 {
		if builtinFun(t, name, n) {
			return true
		}
	}
	return c.hasNamed(t, name)
}

// narrowing returns the smart casts implied by cond being true or false.
func (c *Checker) narrowing(cond syntax.Expr) (whenTrue, whenFalse []narrow) {
	leaf := false
	switch x := cond.(type) {
	case *syntax.Binary:
		leaf = x.Op == syntax.EQ || x.Op == syntax.NE || x.Op == syntax.EQ_EQ_EQ || x.Op == syntax.NOT_EQ_EQ
	case *syntax.Call, *syntax.Is:
		leaf = true
	}
	if leaf {
		// What it says of a var it assigns, x of x?.let { x = null; 1 }
		// != null, was true of its old value.
		if vars := c.watchedVars(cond); len(vars) > 0 {
			defer func() { whenTrue, whenFalse = c.notOf(vars, whenTrue), c.notOf(vars, whenFalse) }()
		}
	}
	switch x := cond.(type) {
	case *syntax.Binary:
		switch x.Op {
		case syntax.EQ, syntax.NE, syntax.EQ_EQ_EQ, syntax.NOT_EQ_EQ:
			defer func() { whenTrue, whenFalse = c.equalExclusions(x, whenTrue, whenFalse) }()
			if t, f, ok := c.boolEquality(x); ok {
				return t, f
			}
			var k narrow
			ok := false
			ynull, xnull := isNull(x.Y), isNull(x.X)
			if ynull {
				k, ok = c.chainKeyOf(x.X)
			} else if xnull {
				k, ok = c.chainKeyOf(x.Y)
			}
			var n, null []narrow
			if ynull {
				null = c.callNull(x.X)
			} else if xnull {
				null = c.callNull(x.Y)
			}
			if ok && types.MayBeNull(k.t) {
				kn := k
				kn.t = types.NullType
				null = append([]narrow{kn}, null...)
				base := k.t
				if id, isID := unchecked(nullSide(x)).(*syntax.Ident); isID {
					if v := c.narrowSym(id); v != nil {
						base = v.Type
					}
				}
				if t, ok := nonNullOf(k.t, base); ok {
					k.t = t
				} else {
					k.t = k.t.NonNull()
				}
				n = append([]narrow{k}, c.impliedBy(k)...)
			} else if ok && k.t.K == types.Invalid && !k.t.D {
				k.t = errorNotNull // see reportedError
				n = []narrow{k}
			}
			// a?.b != null, and a?.b == c where c isn't null, mean that a
			// isn't null.
			eq := x.Op == syntax.EQ || x.Op == syntax.EQ_EQ_EQ
			switch {
			case ynull:
				n = append(append(append(n, c.safeReceivers(x.X)...), c.safeCastNonNull(x.X)...), c.callNonNull(x.X)...)
			case xnull:
				n = append(append(append(n, c.safeReceivers(x.Y)...), c.safeCastNonNull(x.Y)...), c.callNonNull(x.Y)...)
			case equalsNonNull(x.Y.Type()):
				n, eq = c.equalNonNull(x.X), !eq
			case equalsNonNull(x.X.Type()):
				n, eq = c.equalNonNull(x.Y), !eq
			}
			whenTrue, whenFalse = null, n
			if !eq {
				whenTrue, whenFalse = n, null
			}
			// a === b, where b is of a class type, means that a is one too:
			// ktor's prev === this.
			if same := c.identNarrows(x); len(same) > 0 {
				if x.Op == syntax.EQ_EQ_EQ || x.Op == syntax.EQ {
					whenTrue = append(whenTrue[:len(whenTrue):len(whenTrue)], same...)
				} else {
					whenFalse = append(whenFalse[:len(whenFalse):len(whenFalse)], same...)
				}
			}
			// f(x) == Unit: f returned, whichever way the comparison
			// goes, so its contract's returns() implies holds.
			for _, side := range []syntax.Expr{x.X, x.Y} {
				if call, ok := side.(*syntax.Call); ok {
					if ran := c.contractNarrows(call, -1); len(ran) > 0 {
						whenTrue = append(whenTrue[:len(whenTrue):len(whenTrue)], ran...)
						whenFalse = append(whenFalse[:len(whenFalse):len(whenFalse)], ran...)
					}
				}
			}
			if len(whenTrue) == 0 && len(whenFalse) == 0 {
				return nil, nil
			}
			return whenTrue, whenFalse
		case syntax.AND:
			// The right side ran if a && b is true: its x!! smart-cast x.
			// The right side's smart casts build on the left's: x is Int?
			// && x != null makes x an Int. As in K2, the right side of
			// false && b, which never runs, smart-casts nothing.
			if lit, ok := x.X.(*syntax.BoolLit); ok && !lit.Val {
				return nil, nil
			}
			t1, f1 := c.narrowing(x.X)
			c.push()
			c.scope.narrows = t1
			t2, f2 := c.narrowing(x.Y)
			c.pop()
			if neverCompletes(x.Y) {
				// a != null && throw E() is never true: as in K2, code
				// that can't be reached learns nothing.
				return nil, c.eitherNarrows(f1, t1, f2)
			}
			if isTrue(x.Y) {
				// a && true is false where a is.
				return t1, f1
			}
			// So did its casts: x of b && x as Boolean.
			return append(append(t1[:len(t1):len(t1)], t2...), c.ranNarrows(x.Y)...), c.eitherNarrows(f1, t1, f2)
		case syntax.ELVIS:
			return c.elvisNarrowing(x)
		case syntax.OR:
			if isTrue(x.X) {
				return nil, nil // as false && b
			}
			if lit, ok := x.X.(*syntax.BoolLit); ok && !lit.Val {
				return c.narrowing(x.Y) // false || b is b
			}
			t1, f1 := c.narrowing(x.X)
			c.push()
			c.scope.narrows = f1
			t2, f2 := c.narrowing(x.Y)
			c.pop()
			if neverCompletes(x.Y) {
				return c.eitherNarrows(t1, f1, t2), nil // never false
			}
			if lit, ok := x.Y.(*syntax.BoolLit); ok && !lit.Val {
				return t1, f1 // a || false is true where a is
			}
			return c.eitherNarrows(t1, f1, t2), append(append(f1[:len(f1):len(f1)], f2...), c.ranNarrows(x.Y)...)
		}
	case *syntax.Unary:
		if x.Op == syntax.NOT {
			t, f := c.narrowing(x.X)
			return f, t
		}
	case *syntax.Call:
		if m, ok := x.Fun.(*syntax.Member); ok && x.Builtin == types.Bitwise && m.Name == "not" && len(x.Args) == 0 && !m.Safe {
			// x.not() is !x.
			t, f := c.narrowing(m.X)
			return f, t
		}
		// A contract's returns(true) implies (p != null), and false.
		// A Boolean returned is not null either: returnsNotNull()'s.
		nn := c.contractNarrows(x, 2)
		return append(c.contractNarrows(x, 1), nn...), append(c.contractNarrows(x, 0), nn...)
	case *syntax.NotNull:
		if call, ok := x.X.(*syntax.Call); ok {
			// if (f(v)!!): what f's Boolean? result says, as f(v)'s.
			return c.narrowing(call)
		}
	case *syntax.Ident:
		if x.Member == nil && x.Sym != nil {
			if n, ok := c.conds[x.Sym]; ok {
				return c.stableConds(n[0]), c.stableConds(n[1])
			}
			// A local that holds the value of a Boolean val (see alias),
			// as b of val a = x != null; var b = a, says what it says.
			for _, p := range c.sameAs[flowKey{sym: x.Sym}] {
				if n, ok := c.conds[p.sym]; ok && p.recv == nil && (!isVar(x.Sym) || c.varStable(x.Sym, types.Type{})) {
					return c.stableConds(n[0]), c.stableConds(n[1])
				}
			}
		}
	case *syntax.Is:
		if x.Target.K == types.Invalid {
			return nil, nil
		}
		// a?.b is T, for a T that isn't nullable, tells a isn't null.
		var safe []narrow
		if !types.MayBeNull(x.Target) {
			safe = c.safeReceivers(x.X)
		}
		k, ok := c.narrowKeyOf(x.X)
		if m, isSafe := x.X.(*syntax.Member); isSafe && m.Safe && !ok && safe != nil {
			k, ok = c.memberKey(m, m.Type())
		}
		if !ok {
			if x.Not {
				return nil, safe
			}
			return safe, nil
		}
		// Where it isn't a T, a when over it needn't cover T.
		// Where it is one, nor the subclasses of a sealed class that T
		// isn't.
		var not []narrow
		var others *coverage
		base := k.t
		if v := c.narrowSym(x.X); v != nil {
			base = v.Type
		}
		if t, ok := notIsNarrowed(k.t, base, x.Target); ok {
			kn := k
			kn.t = t
			not = []narrow{kn}
			if t.K != types.Null && t.K != types.Nothing {
				not = append(not, c.impliedBy(kn)...)
			}
		}
		if x.Target.K == types.Class && !types.MayBeNull(x.Target) {
			not = c.excluding(nil, k, &coverage{classes: []*types.ClassSym{x.Target.C}})
			// Not of a check that makes an intersection, A & B for
			// an A checked to be a B: as in K2, a when over it must
			// cover A's or B's sealed subclasses, whichever of the two
			// it covers, without the ones of A's that aren't B's.
			if st := k.t.NonNull(); st.K == types.Class && x.Target.C.IsSubclassOf(st.C) {
				others = &coverage{}
				otherSubclasses(st.C, x.Target.C, others)
			}
		}
		key := k
		k.t = c.isNarrowed(k.t, x.Target)
		k.javaClass = x.Ref != nil && strings.HasPrefix(x.Ref.Name, "java.") && x.Target.K == types.Class && builtinJava[x.Target.C.Name]
		n := append(append([]narrow{k}, safe...), c.impliedBy(k)...)
		if others != nil && !others.empty() {
			n = c.excluding(n, key, others)
		}
		if x.Not {
			return not, n
		}
		return n, not
	}
	return nil, nil
}

// notIsNarrowed returns the type of a value of type cur, declared as
// base, where it isn't a target, as K2's data flow has it: x is Any is
// x != null, so it is null; and a null is a T?, x is Any? among those,
// so what isn't one isn't null. ok is false where that tells nothing.
func notIsNarrowed(cur, base, target types.Type) (t types.Type, ok bool) {
	if !types.MayBeNull(cur) || cur.K == types.Invalid || target.P || target.K == types.Null || target.K == types.Invalid {
		return cur, false
	}
	switch {
	case target.K == types.Any && !target.N:
		if cur.K == types.Null {
			return cur, false
		}
		return types.NullType, true
	case target.N:
		return nonNullOf(cur, base)
	}
	return cur, false
}

// neverCompletes reports whether x, a checked expression, is of type
// Nothing: it always jumps.
func neverCompletes(x syntax.Expr) bool {
	t := x.Type()
	return t.K == types.Nothing && !t.N
}

// nullSide returns the side of x, a comparison with null, that isn't
// the null literal.
func nullSide(x *syntax.Binary) syntax.Expr {
	if isNull(x.Y) {
		return x.X
	}
	return x.Y
}

// nonNullOf returns the type of a value of type cur, declared as base,
// where it isn't null. Of one known to be null, which K2 types as an
// intersection with Nothing whose members are base's, kc, without such
// intersections, takes base's non-null type, so that its members stay
// resolved; ok is false if there is none.
func nonNullOf(cur, base types.Type) (types.Type, bool) {
	if cur.K != types.Null {
		return cur.NonNull(), true
	}
	if base.K == types.Invalid || base.K == types.Null || base.K == types.Nothing || !types.MayBeNull(base) {
		return cur, false
	}
	return base.NonNull(), true
}

// identNarrows returns the smart casts that a === b or a !== b implies
// for either side, when the other is of a class type that it isn't known
// to be. A side of a generic type keeps it when the other's is generic
// too: kotlinc's smart cast to both would let inference pick either's
// arguments, as bar(s) for s: Slice<X> === SL0: Slice<A>, where kc's
// would replace them.
func (c *Checker) identNarrows(x *syntax.Binary) []narrow {
	switch x.Op {
	case syntax.EQ_EQ_EQ, syntax.NOT_EQ_EQ:
	case syntax.EQ, syntax.NE:
		// As K2, a == b does too where a's equals is safe for smart casts
		// (see safeEquals): ts[0] == a, for a String ts[0], makes a a
		// String.
		if !c.safeEquals(x.X.Type()) {
			return nil
		}
	default:
		return nil
	}
	var out []narrow
	for _, side := range [][2]syntax.Expr{{x.X, x.Y}, {x.Y, x.X}} {
		ot := side[1].Type()
		if ot.K != types.Class && ot.K != types.String && ot.K != types.TypeParam || types.MayBeNull(ot) {
			continue
		}
		k, ok := c.narrowKeyOf(side[0])
		if !ok || c.subtype(k.t, ot) {
			continue
		}
		if c.funcSubNarrow(k.t, ot) {
			k.t = ot
			out = append(out, k)
			continue
		}
		if ot.A != nil && k.t.K == types.Class && k.t.A != nil {
			// Both generic: the value is both, Box<T> & Box<A>, as K2's
			// smart cast is an intersection with the declared type. The
			// declared type first, which inference, without K2's forks
			// over the components, takes: bar(s) for an expected X?.
			if ot.K != types.Class || ot.C.Func || k.t.C.Func || types.MayBeNull(k.t) {
				continue
			}
			k.t = c.intersect([]types.Type{k.t, ot})
			out = append(out, k)
			continue
		}
		k.t = c.narrowedTo(k.t, ot, false)
		out = append(out, k)
	}
	return out
}

// safeEquals reports whether a value of type t is equal only to values of
// its type, as K2's EqualsOverrideContract.SAFE_FOR_SMART_CAST has it: a
// String, or a final class that, with its superclasses, doesn't declare
// equals, an enum among them, but not a data class, whose equals kotlinc
// only trusts for exhaustiveness, nor a Float or a Double. A type
// parameter or an intersection is if one of its types is.
func (c *Checker) safeEquals(t types.Type) bool {
	t = t.Flex().NonNull()
	switch {
	case t.K == types.String:
		return true
	case t.IsIntersection():
		return slices.ContainsFunc(t.Param().Bounds, c.safeEquals)
	case t.IsParam():
		return slices.ContainsFunc(t.Param().Bounds, c.safeEquals)
	case t.K != types.Class || t.C == nil:
		return false
	}
	cls := t.C
	cls.Ensure()
	if !cls.Final || cls.Abstract || cls.Interface || cls.Sealed || cls.Data || cls.Array {
		return false
	}
	for k := cls; k != nil; k = k.Super {
		k.Ensure()
		switch k.Name {
		case "java/lang/Object", "java/lang/Enum", "kotlin/Any", "kotlin/Enum":
			continue
		}
		for _, f := range k.Methods["equals"] {
			if len(f.Params) == 1 && !f.Static {
				return false
			}
		}
	}
	return true
}

// contractNarrows returns the smart casts that the contract of a call's
// function implies when it returns a value (1: true, 0: false, -1: at
// all, 2: not null, 3: null): the arguments it says aren't null, or are
// of a type, then.
func (c *Checker) contractNarrows(x *syntax.Call, returns int) []narrow {
	f := x.Sym
	if f == nil || len(f.Implies) == 0 {
		return nil
	}
	var out []narrow
	// Each effect builds on those before it: myRequireAnd(x is B, x is
	// C) makes x a B & C.
	c.push()
	defer c.pop()
	for _, e := range f.Implies {
		if e.Returns != returns {
			continue
		}
		c.scope.narrows = out
		var arg syntax.Expr
		if e.Param < 0 {
			m, ok := x.Fun.(*syntax.Member)
			if !ok || m.Safe {
				continue
			}
			arg = m.X
		} else if i := e.Param - len(f.Params); i >= 0 {
			if i < len(x.Ctx) {
				arg = x.Ctx[i]
			}
		} else {
			for k, a := range x.Args {
				i := k
				if k < len(x.ArgMap) {
					i = x.ArgMap[k]
				}
				if i == e.Param {
					arg = a
				}
			}
		}
		if arg == nil {
			continue
		}
		if e.Holds {
			whenTrue, whenFalse := c.narrowing(arg) // check(x is T)
			if e.Negated {
				whenTrue = whenFalse // implies (!p)
			}
			out = append(out, whenTrue...)
			continue
		}
		if e.Null {
			// The argument is null: of type Nothing?, as after x == null.
			if k, ok := c.chainKeyOf(arg); ok && types.MayBeNull(k.t) {
				k.t = types.NullType
				out = append(out, k)
			}
			continue
		}
		if e.Is == nil {
			// The argument isn't null, nor are the receivers of its safe
			// calls: !x?.y.isNullOrEmpty() means x isn't null either.
			out = append(out, c.equalNonNull(arg)...)
			continue
		}
		k, ok := c.narrowKeyOf(arg)
		if !ok {
			continue
		}
		is := *e.Is
		if is.HasParams() && len(x.Reified) == len(f.TypeParams) {
			// The call's type arguments: value is R for its R.
			sub := types.Subst{}
			for i, tp := range f.TypeParams {
				sub[tp] = types.Arg{T: x.Reified[i]}
			}
			is = sub.Apply(is)
		}
		is = c.contractArgs(f, e.Param, k.t, is)
		own := func(t types.Type) bool {
			return t.Has(func(u types.Type) bool { return u.K == types.TypeParam && slices.Contains(f.TypeParams, u.Param()) })
		}
		if own(is) && is.K == types.Class && len(is.C.TypeParams) > 0 {
			// this is Error, of a NetRequestStatus<T>: the arguments
			// are the argument's, as for x is Error.
			if t, known, _ := c.bareArgs(is.C, &syntax.TypeRef{Nullable: is.N}, k.t); known {
				is = t
			}
		}
		if !own(is) && !c.subtype(k.t, is) {
			k.t = c.isNarrowed(k.t, is)
			out = append(out, k)
		}
	}
	return out
}

// asserted returns the smart casts of the x!! that evaluating x runs
// whatever it gives: those of its operands, receivers and arguments, but
// not those after && or || or ?:, or in lambdas.
func (c *Checker) asserted(x syntax.Expr) []narrow {
	var out []narrow
	var walk func(x syntax.Expr)
	walk = func(x syntax.Expr) {
		switch x := x.(type) {
		case *syntax.NotNull:
			walk(x.X)
			if k, ok := c.narrowKeyOf(x.X); ok && types.MayBeNull(k.t) {
				k.t = k.t.NonNull()
				out = append(out, k)
			}
			// a?.b!!: a isn't null either.
			out = append(out, c.safeReceivers(x.X)...)
		case *syntax.Member:
			if !x.Safe {
				walk(x.X)
			}
		case *syntax.Call:
			if m, ok := x.Fun.(*syntax.Member); ok && !m.Safe {
				walk(m.X)
			}
			for _, a := range x.Args {
				if _, ok := a.(*syntax.Lambda); !ok {
					walk(a)
				}
			}
		case *syntax.Binary:
			walk(x.X)
			if x.Op != syntax.AND && x.Op != syntax.OR && x.Op != syntax.ELVIS {
				walk(x.Y)
			}
		case *syntax.Unary:
			walk(x.X)
		case *syntax.Is:
			walk(x.X)
		}
	}
	walk(x)
	return out
}

// safeCallOf reports whether x is a safe call, x?.f() or x?.p.
func safeCallOf(x syntax.Expr) bool {
	switch x := x.(type) {
	case *syntax.Member:
		return x.Safe
	case *syntax.Call:
		m, ok := x.Fun.(*syntax.Member)
		return ok && m.Safe
	}
	return false
}

// safeReceivers returns the smart casts implied by a chain of safe
// calls, a?.b?.c(), being non-null: none of its receivers is null.
func (c *Checker) safeReceivers(x syntax.Expr) []narrow {
	var recv syntax.Expr
	switch x := x.(type) {
	case *syntax.Member:
		if x.Safe {
			recv = x.X
		}
	case *syntax.Call:
		if m, ok := x.Fun.(*syntax.Member); ok && m.Safe {
			recv = m.X
		}
	}
	if recv == nil {
		return nil
	}
	return append(c.equalNonNull(recv), c.safeArgCasts(x)...)
}

// equalNonNull returns the smart casts implied by x being equal to a
// value that isn't null: neither x nor the receivers of its safe calls
// are null.
// elvisNonNull returns the smart casts of x not being null, the left side
// of an elvis whose right side jumps: those of a as well for an a ?: n or
// n ?: a whose n is always null, as in a ?: b?.let { return it } ?: return.
func (c *Checker) elvisNonNull(x syntax.Expr) []narrow {
	out := append(c.equalNonNull(x), c.notNullCast(x)...)
	if e, ok := x.(*syntax.Binary); ok && e.Op == syntax.ELVIS {
		switch {
		case isNull(e.Y):
			out = append(out, c.elvisNonNull(e.X)...)
		case isNull(e.X):
			out = append(out, c.elvisNonNull(e.Y)...)
		}
	}
	return out
}

func (c *Checker) equalNonNull(x syntax.Expr) []narrow {
	if as, ok := x.(*syntax.As); ok && as.Safe {
		// (a as? T) isn't null: a is a T, as after (a as? T)?.f() != null.
		if k, ok := c.castNarrow(as); ok {
			k.t = k.t.NonNull()
			return append([]narrow{k}, c.impliedBy(k)...)
		}
		// Nor is a, nor the receivers of its safe calls: x of
		// x?.bar as? String ?: return, as in K2.
		return c.equalNonNull(as.X)
	}
	n := c.safeReceivers(x)
	k, ok := c.narrowKeyOf(x)
	if m, safe := x.(*syntax.Member); safe && m.Safe {
		k, ok = c.memberKey(m, m.Type())
	}
	if ok && types.MayBeNull(k.t) {
		k.t = k.t.NonNull()
		n = append(append(n, k), c.impliedBy(k)...)
	} else if ok && k.t.K == types.Invalid && !k.t.D {
		k.t = errorNotNull // see reportedError
		n = append(n, k)
	}
	// f(x) isn't null: what f's returnsNotNull() implies, as in
	// notEqualsNull(nullWhenNotString(x)).
	return append(n, c.callNonNull(x)...)
}

// callNonNull returns what call x not being null tells: what its
// function's returnsNotNull() implies, as of a?.f(s) != null.
func (c *Checker) callNonNull(x syntax.Expr) []narrow {
	if call, ok := x.(*syntax.Call); ok {
		return c.contractNarrows(call, 2)
	}
	return nil
}

// callNull returns what call x being null tells: what its function's
// returns(null) implies. Not of a safe call, a?.f(s), which is null
// where a is too.
func (c *Checker) callNull(x syntax.Expr) []narrow {
	call, ok := x.(*syntax.Call)
	if !ok || safeCallOf(call) {
		return nil
	}
	return c.contractNarrows(call, 3)
}

// safeCastNonNull returns what a as? T not being null tells: a is a T.
func (c *Checker) safeCastNonNull(x syntax.Expr) []narrow {
	if as, ok := x.(*syntax.As); ok && as.Safe {
		return c.equalNonNull(as)
	}
	return nil
}

// notOf returns the smart casts of ns but those of vars and their
// properties.
func (c *Checker) notOf(vars []*types.VarSym, ns []narrow) []narrow {
	return slices.DeleteFunc(slices.Clone(ns), func(n narrow) bool {
		return slices.Contains(vars, n.recv) || n.recv == nil && slices.Contains(vars, n.sym)
	})
}

// localKey returns the local the smart cast k is of, or nil if k is of a
// property.
func (c *Checker) localKey(k narrow) *types.VarSym {
	if k.recv != nil || k.sym.Kind != types.Local {
		return nil
	}
	return k.sym
}

// nonNull reports whether a checked expression's type has no null.
func nonNull(t types.Type) bool {
	return t.K != types.Invalid && !types.MayBeNull(t)
}

// equalsNonNull reports whether being equal to a value of type t makes the
// other side of the equality non-null. kotlinc counts a platform type, such
// as that of a Java static field, as non-null here, so when (r?.action)
// { File.separator -> } smart casts r.
func equalsNonNull(t types.Type) bool {
	return nonNull(t) || t.P
}

// receiver checks the receiver of a member access or member call.
func (c *Checker) receiver(x syntax.Expr, safe bool, name string, p syntax.Pos) (types.Type, bool) {
	t := c.value(x, noWant)
	if t.K == types.Invalid {
		return t, false
	}
	if types.MayBeNull(t) && !t.P && !safe {
		c.errorf(p, "only safe (?.) or non-null asserted (!!.) calls are allowed on a nullable receiver of type %s", t)
		return types.InvalidType, false
	}
	return t.NonNull(), true
}

// safeInvoke checks x?.f(args), for a property f of function type, with x
// checked: x?.let { r -> r.f(args) }, which evaluates the arguments only
// if x isn't null.
func (c *Checker) safeInvoke(x *syntax.Call, fn *syntax.Member, want types.Type, sink *system) types.Type {
	r := &syntax.Ident{Name: "$receiver"}
	r.P = fn.X.Pos()
	m := &syntax.Member{X: r, Name: fn.Name}
	m.P = fn.P
	inner := &syntax.Call{Fun: m, Args: x.Args, Names: x.Names}
	inner.P = x.P
	lam := &syntax.Lambda{Arrow: true, Params: []*syntax.LambdaParam{{P: r.P, Name: r.Name}},
		Body: &syntax.Block{P: x.P, Stmts: []syntax.Stmt{&syntax.ExprStmt{X: inner}}}}
	lam.P = x.P
	let := &syntax.Member{X: checked(fn.X, fn.X.Type()), Name: "let", Safe: true}
	let.P = fn.P
	x.Fun, x.Args, x.Names = let, []syntax.Expr{lam}, nil
	return c.call(x, want, sink)
}

// builtinBound returns x as the bound of its type t, and the bound, if t
// is a type parameter bounded by a built-in type or a value class, whose
// members and operators x has: x + 1 for T : Int, and the unboxed value
// that a value class's methods take. Otherwise it returns x and t.
func builtinBound(x syntax.Expr, t types.Type) (syntax.Expr, types.Type) {
	if t.K != types.TypeParam {
		return x, t
	}
	b := t.Param().Bound().NonNull()
	for b.K == types.TypeParam {
		// T : X, X : Char.
		b = b.Param().Bound().NonNull()
	}
	if !b.IsPrimitive() && b.K != types.String && !types.IsValue(b) {
		return x, t
	}
	if types.MayBeNull(t) {
		b = b.Nullable()
	}
	return checked(x, b), b
}

// intLitExpr reports whether x is an integer literal without a suffix,
// possibly negated.
func intLitExpr(x syntax.Expr) bool {
	switch x := x.(type) {
	case *syntax.IntLit:
		return !x.Long && !x.Unsigned
	case *syntax.Unary:
		return x.Op == syntax.MINUS && intLitExpr(x.X)
	}
	return false
}

// signedIntLit reports whether x is an integer literal, with a sign or
// not, which only an expected type makes another type than Int or Long.
func signedIntLit(x syntax.Expr) bool {
	switch x := x.(type) {
	case *syntax.IntLit:
		return !x.Unsigned
	case *syntax.Unary:
		return (x.Op == syntax.MINUS || x.Op == syntax.PLUS) && signedIntLit(x.X)
	}
	return false
}

// intersectionView returns the type of an intersection t, of smart
// casts, whose member or extension name a call or a property read on it
// finds, where kc's
// lookup in t itself wouldn't find it: a built-in type's, String's or
// Int's, that none of its class types has, wherever it is in t, as
// x.length after x is A and x is String, or of an Int & String x after
// x is String and x is Int; and o.trim() of a String & T is a String,
// not CharSequence.trim()'s CharSequence, but not o.compareTo(p) for
// T : Comparable<T>, whose bound has it too. Of an intersection whose
// first type is a built-in one without name, its first class type with
// it: x.foo() after x is A and x is Int.
func (c *Checker) intersectionView(t types.Type, name string) (types.Type, bool) {
	if !t.IsIntersection() {
		return types.InvalidType, false
	}
	bounds := t.Param().Bounds
	others := func(i int) bool {
		for j, b := range bounds {
			if j != i && !finalBuiltin(b.NonNull()) && c.hasNamed(b, name) {
				return true
			}
		}
		return false
	}
	builtin := func(b types.Type) bool {
		// String's length and Char's code are built-in properties.
		return b == types.StringType && name == "length" || b == types.CharType && name == "code" || c.builtinNamed(b, name)
	}
	for i, b := range bounds {
		if b = b.NonNull(); finalBuiltin(b) && builtin(b) {
			if others(i) {
				break
			}
			return b, true
		}
	}
	if first := bounds[0].NonNull(); finalBuiltin(first) && !builtin(first) {
		for _, b := range bounds[1:] {
			if b.K == types.Class && c.hasNamed(b, name) {
				return b.NonNull(), true
			}
		}
	}
	return types.InvalidType, false
}

// callReceiver checks the receiver of a member call, expecting want. A
// nullable receiver can be called without ?. if extensions for nullable
// receivers take it, as String?.isNullOrEmpty() does: callReceiver
// returns them, and the receiver's type stays nullable.
func (c *Checker) callReceiver(fn *syntax.Member, want types.Type) (types.Type, []callee, bool) {
	c.entriesQualifier(fn.X)
	vt := c.value(fn.X, want)
	if id, ok := fn.X.(*syntax.Ident); ok && id.Sym != nil && len(c.stubs) > 0 && c.stub(vt.Flex()) == nil {
		// A variable declared with a builder's variable for its type and
		// smart-cast by an assignment is a PTV & TargetType: a member
		// call on it fixes the variable, as one on the variable does.
		c.fixedStub(id.Sym.Type)
	}
	t := c.fixedStub(c.declaredIfNull(fn.X, vt))
	if t.K == types.Invalid {
		if reportedError(t) && !anyMember[fn.Name] { // K2 finds Any's members
			c.errorf(fn.P, "unresolved reference: %s", fn.Name)
			return types.InvalidType, nil, false
		}
		return t, nil, false
	}
	if c.unfixedReceiver(fn.X, t, fn.Name, fn.P) {
		return types.InvalidType, nil, false
	}
	t = c.castView(fn.X, t, fn.Name)
	if b, ok := c.intersectionView(t, fn.Name); ok {
		if types.MayBeNull(t) {
			b = b.Nullable()
		}
		fn.X, t = checked(fn.X, b), b
	}
	if vt.K == types.Null && t.K != types.Null && !fn.Safe && c.extCallees(fn.Name, t) == nil {
		// A variable known to be null is a Nothing?, which any extension
		// of a nullable receiver takes: i.foo() for a String?.foo()
		// after i == null, of an Int? i.
		if cands := c.extCallees(fn.Name, vt); len(cands) > 0 {
			fn.X = checked(fn.X, vt)
			return vt, cands, true
		}
	}
	if st, ok := c.suspendRecv(fn, t); ok {
		// A reference receiver is suspend converted for a suspend
		// function type's extension: (::f).ext() for
		// fun (suspend () -> Unit).ext().
		fn.X = c.suspendConverted(fn.X, st)
		return st, nil, true
	}
	if c.extCallees(fn.Name, t) == nil {
		fn.X, t = builtinBound(fn.X, t)
	}
	if types.MayBeNull(t) && !t.P && !fn.Safe {
		if cands := c.extCallees(fn.Name, t); len(cands) > 0 {
			return t, cands, true
		}
		if fn.Name == "toString" {
			return t, nil, true // Any?.toString(), a built-in extension
		}
		if fn.Name == "plus" && (t.NonNull() == types.StringType || t.K == types.Null) {
			return t, nil, true // String?.plus(Any?), a built-in extension
		}
		c.errorf(fn.P, "only safe (?.) or non-null asserted (!!.) calls are allowed on a nullable receiver of type %s", t)
		return t, nil, false
	}
	return t.NonNull(), nil, true
}

// inaccessibleOuters returns the classes around the current one whose
// instances its code has no receivers of, innermost first: those outside
// a nested class that isn't inner. A companion object isn't such a class
// for kotlinc, which reports an outer class's member there unresolved.
func (c *Checker) inaccessibleOuters() []*types.ClassSym {
	var out []*types.ClassSym
	cut := false
	for k := c.cls; k != nil; k = k.Outer {
		if cut {
			out = append(out, k)
		} else if o := k.Outer; o != nil && !k.Inner && !k.Local && !k.Anonymous && o.Companion != k {
			cut = true
		}
	}
	return out
}

// inaccessibleOuterCall reports at p a call of function name, with a
// receiver of type *recv or else none, that resolves only to a function
// of a class of inaccessibleOuters, which needs an instance of it: a
// member, a member extension taking the receiver, or, without one, an
// extension of the class. As kotlinc, which reports the call
// (INACCESSIBLE_OUTER_CLASS_RECEIVER), it has the function's result
// type, if the call doesn't infer it. It reports whether there is one.
func (c *Checker) inaccessibleOuterCall(p syntax.Pos, name string, recv *types.Type) (types.Type, bool) {
	for _, o := range c.inaccessibleOuters() {
		var fs []*types.FunSym
		if recv == nil {
			for _, f := range c.javaMethods(o, name) {
				if !f.Static {
					fs = append(fs, f)
				}
			}
		}
		for _, f := range o.ExtMethods[name] {
			// Without a receiver, an implicit one's, as with(s) { f() }.
			if recv == nil || f.Recv.K != types.Invalid && c.recvFits(*recv, f.Recv) {
				fs = append(fs, f)
			}
		}
		if len(fs) == 0 && recv == nil {
			fs = calleeFuns(c.extCallees(name, o.This()))
		}
		if len(fs) == 0 {
			continue
		}
		c.errorf(p, "outer class '%s' of non-inner class cannot be used as receiver", o.DisplayName())
		if len(fs) > 1 {
			return types.InvalidType, true
		}
		f := fs[0]
		if ensureFun(f); !f.Resolved || len(f.TypeParams) > 0 || f.Ret.K == types.Invalid || f.Ret.HasParams() || f.Ret.HasVars() {
			return types.InvalidType, true
		}
		return f.Ret, true
	}
	return types.InvalidType, false
}

// inaccessibleOuterProp reports the read x of a property of a class of
// inaccessibleOuters, which needs an instance of it, and returns the
// property's type, as kotlinc's. It reports whether there is one.
func (c *Checker) inaccessibleOuterProp(x *syntax.Ident) (types.Type, bool) {
	for _, o := range c.inaccessibleOuters() {
		p := c.userProp(o, x.Name)
		if p == nil {
			continue
		}
		c.errorf(x.P, "outer class '%s' of non-inner class cannot be used as receiver", o.DisplayName())
		if ensureVar(p); !p.Resolved || p.Type.HasParams() || p.Type.HasVars() {
			return types.InvalidType, true
		}
		return p.Type, true
	}
	return types.InvalidType, false
}

// nullableImplicit returns the type of the innermost implicit receiver
// whose type is nullable and whose non-null type has a member or an
// extension named name, a property if call is nil, else a function call
// may call: where nothing else has the name, kotlinc resolves it on that
// receiver and reports the unsafe call, as length in with(s) { length }
// for a String? s.
func (c *Checker) nullableImplicit(name string, call *syntax.Call) (types.Type, bool) {
	for i := len(c.recvs) - 1; i >= 0; i-- {
		t := c.scope.narrowed(c.recvs[i].sym)
		if t.K == types.Invalid || !types.MayBeNull(t) || t.P {
			continue
		}
		nn := t.NonNull()
		if call != nil {
			if cls := c.classOf(nn); cls != nil && len(c.javaMethods(cls, name)) > 0 || len(c.extCallees(name, nn)) > 0 {
				return t, true
			}
			continue
		}
		if c.hasPropOn(nn, name) {
			return t, true
		}
	}
	return types.InvalidType, false
}

// hasPropOn reports whether a receiver of type t, not nullable, has a
// property named name: a read of it checks without errors.
func (c *Checker) hasPropOn(t types.Type, name string) bool {
	n := len(c.errs)
	_, ok := c.propOn(t, name)
	c.errs = c.errs[:n]
	return ok
}

// propOn checks the read of property name on a receiver of type t, and
// returns its type and whether it checks without errors, which it leaves
// for the caller to roll back.
func (c *Checker) propOn(t types.Type, name string) (types.Type, bool) {
	r := &syntax.Ident{Name: "$receiver"}
	m := &syntax.Member{X: checked(r, t), Name: name}
	n := len(c.errs)
	pt := c.memberOn(m, t)
	return pt, len(c.errs) == n && pt.K != types.Invalid
}

// unsafeImplicitProp reports the read x of a property of the implicit
// receiver of nullable type rt, which nullableImplicit found, and returns
// the property's type, as kotlinc's on the non-null receiver.
func (c *Checker) unsafeImplicitProp(x *syntax.Ident, rt types.Type) types.Type {
	c.errorf(x.P, "only safe (?.) or non-null asserted (!!.) calls are allowed on a nullable receiver of type %s", rt)
	n := len(c.errs)
	t, ok := c.propOn(rt.NonNull(), x.Name)
	c.errs = c.errs[:n]
	if !ok {
		return types.InvalidType
	}
	return t
}

// unsafeCallResult returns the type of call x, x.name(...), whose receiver is of
// nullable type rt, an error kotlinc reports while it still resolves the
// call on the non-null receiver, which then has its function's result
// type: run { x.foo() } infers its R from it. kc takes the result type
// that all the receiver's functions of the name share, if it doesn't
// depend on their type parameters, and otherwise the error type.
func (c *Checker) unsafeCallResult(x *syntax.Call, rt types.Type, name string) types.Type {
	if rt.K == types.Invalid {
		// Or of an error type's: K2 finds Any's members, so that
		// unresolved.toString() is a String.
		switch {
		case len(c.errs) == 0 || x.Names != nil:
		case name == "toString" && len(x.Args) == 0:
			return types.StringType
		case name == "hashCode" && len(x.Args) == 0:
			return types.IntType
		case name == "equals" && len(x.Args) == 1:
			return types.BoolType
		}
		return types.InvalidType
	}
	rt = rt.NonNull()
	var ms []*types.FunSym
	if cls := c.classOf(rt); cls != nil {
		ms = c.javaMethods(cls, name)
	}
	ext := len(ms) == 0
	if ext {
		// Or its extensions', as foo() of fun J.foo() in
		// with(j) { foo() } for a J? j.
		ms = calleeFuns(c.extCallees(name, rt))
	}
	if len(ms) == 0 {
		return types.InvalidType
	}
	res := types.InvalidType
	for _, f := range ms {
		if ensureFun(f); !f.Resolved || len(f.TypeParams) > 0 || f.Ret.K == types.Invalid {
			return types.InvalidType
		}
		t := f.Ret
		if !ext {
			t = c.memberSubst(rt, f.Owner).Apply(f.Ret)
		}
		classParam := func(u types.Type) bool {
			if !u.IsParam() {
				return false
			}
			_, ok := u.Param().Owner.(*types.ClassSym)
			return ok
		}
		if t.HasVars() || t.Has(classParam) {
			return types.InvalidType
		}
		if res.K != types.Invalid && res != t {
			return types.InvalidType
		}
		res = t
	}
	return res
}

func (c *Checker) member(x *syntax.Member) types.Type {
	if cl, ok := x.X.(*syntax.ClassLit); ok && x.Name == "java" && !x.Safe {
		// X::class.java is the Java class, without a KClass.
		t := c.classLit(cl, true)
		cl.SetType(t)
		if t.K == types.Class && t.A != nil && len(t.A.List) == 1 {
			// T of the extension property KClass<T>.java.
			if a := t.A.List[0].T; a.K == types.Class && boxedPlatform[a.C.Name] {
				if c.platformArgs == nil {
					c.platformArgs = map[any]string{}
				}
				c.platformArgs[x] = platformMapped[a.C.Name]
			}
		}
		return t
	}
	if x.Object != nil {
		return types.ClassType(x.Object) // an object, already resolved
	}
	if x.Safe && c.qualifierValue(x.X) {
		c.noteQualifierSafe(x)
		return types.InvalidType
	}
	if id, ok := x.X.(*syntax.Ident); ok && c.starQualifierAmbiguity(id) {
		return types.InvalidType
	}
	c.entriesQualifier(x.X)
	if cls := c.classRef(x); cls != nil && !c.hasStatic(c.classRef(x.X), x.Name) {
		if obj := objectOf(cls); obj != nil {
			c.checkClassVisible(obj, x.P)
			if !c.usable(obj, x.P) {
				return types.InvalidType // a library's internal object
			}
			return objectValue(x, obj) // a.b.Obj, Outer.Companion, kotlin.random.Random
		}
	}
	if pkg := c.packageRef(x.X); pkg != nil {
		// A top-level property by its qualified name: a.b.x.
		v, ok := pkg.Vars[x.Name]
		if !ok {
			c.errorf(x.P, "unresolved reference: %s", x.Name)
			return types.InvalidType
		}
		ensureVar(v)
		x.Prop = v
		x.T = v.Type
		if k, ok := c.narrowKeyOf(x); ok && k.t.K != types.Invalid && k.t != v.Type {
			x.Cast = needsCast(v.Type, k.t) // a smart cast of a.b.x
			return k.t
		}
		return v.Type
	}
	if v := c.libraryPackageVar(x); v != nil {
		// A library's: kotlin.coroutines.coroutineContext.
		x.Prop = v
		if v.Suspend {
			c.suspendCall(x.P, &types.FunSym{Name: v.Name, Suspend: true})
		}
		return v.Type
	}
	if cls := c.classRef(x.X); cls != nil && (objectOf(cls) == nil || c.hasStatic(cls, x.Name)) {
		return c.staticField(x, cls)
	}
	c.rootObject(x.X)
	if id, ok := x.X.(*syntax.Ident); ok && !c.isVar(id.Name) {
		// A constant of a built-in type's companion: Int.MAX_VALUE.
		if obj := c.builtinCompanion(id.Name); obj != nil && c.hasStatic(obj, x.Name) {
			return c.staticField(x, obj)
		}
		if obj := c.builtinCompanion(id.Name); obj != nil && x.Name == "Companion" {
			return objectValue(x, obj) // Int.Companion
		}
	}
	if s, ok := x.X.(*syntax.Super); ok {
		c.superSafeCall(x)
		return c.superMember(x, s)
	}
	if l, ok := x.X.(*syntax.Lambda); ok && l.Ref != nil && x.Name == "isInitialized" && !x.Safe {
		return c.isInitialized(x, l.Ref)
	} else if x.Name == "isInitialized" {
		defer c.lateinitIntrinsic(x)
	}
	if l, ok := x.X.(*syntax.Lambda); ok && l.Ref != nil && x.Name == "name" && !x.Safe {
		// A reference's name is a constant, as with kotlinc: ktor's
		// StackWalkingFailed::failedToCaptureStackFrame.name.
		if c.value(l, noWant).K == types.Invalid {
			return types.InvalidType
		}
		x.Builtin = types.RefName
		if l.Ref.BoundExpr != nil && !effectless(l.Ref.BoundExpr) {
			return types.StringType // (i++)::toString.name evaluates i++
		}
		lit := &syntax.StringLit{Parts: []syntax.TmplPart{{Lit: l.RefName()}}}
		lit.P = l.P
		lit.T = types.StringType
		x.X = lit
		return types.StringType
	}
	xt := c.fixedStub(c.declaredIfNull(x.X, c.value(x.X, noWant)))
	if xt.K == types.Invalid {
		if reportedError(xt) {
			c.errorf(x.P, "unresolved reference: %s", x.Name)
			return types.InvalidType
		}
		return xt
	}
	if c.unfixedReceiver(x.X, xt, x.Name, x.P) {
		return types.InvalidType
	}
	xt = c.castView(x.X, xt, x.Name)
	if b, ok := c.intersectionView(xt, x.Name); ok {
		if types.MayBeNull(xt) {
			b = b.Nullable()
		}
		x.X, xt = checked(x.X, b), b
	}
	x.X, xt = builtinBound(x.X, xt)
	if types.MayBeNull(xt) && !xt.P && !x.Safe {
		// Only an extension property for nullable receivers takes it.
		if t, ok := c.extProp(x, xt); ok {
			return t
		}
		c.errorf(x.P, "only safe (?.) or non-null asserted (!!.) calls are allowed on a nullable receiver of type %s", xt)
		// As kotlinc, the property is still read, on the non-null
		// receiver, and has its type: val s: String = x.length, of a
		// String? x, is a mismatch too, and run { x.length } an Int. If
		// that takes another error, the read is the error type.
		n := len(c.errs)
		if t := c.memberOn(x, xt); len(c.errs) == n {
			return t
		}
		c.errs = c.errs[:n]
		return types.InvalidType
	}
	return c.memberOn(x, xt)
}

// memberOn checks the read of property x.Name on x.X, of type xt, which
// takes it: xt's non-null type does.
func (c *Checker) memberOn(x *syntax.Member, xt types.Type) types.Type {
	rt := c.propBound(x, xt.NonNull())
	var t types.Type
	cls := c.classOf(rt)
	found := true
	switch {
	case rt == types.StringType && x.Name == "length":
		x.Builtin = types.StrLength
		t = types.IntType
	case rt == types.CharType && x.Name == "code":
		x.Builtin = types.CharCode
		t = types.IntType
	case cls != nil && (builtinCompanions[cls.Name] || cls.External && ensured(cls).Object) && c.hasStatic(cls, x.Name) && !x.Safe:
		// A constant of a built-in type's companion, or of an object, on
		// an instance of it, MAX_VALUE in fun Int.Companion.f(): a static
		// field, after the instance's effects.
		x.EvalX = !effectless(x.X)
		return c.staticField(x, cls)
	case cls != nil && cls.External && cls.Value && cls.Underlying != nil && x.Name == cls.Underlying.Name && !c.contextKProp(cls, x.Name):
		// A class-path value class's value: the unboxed value itself,
		// unless it is a property of the name with context parameters.
		x.Prop = cls.Underlying
		t = c.memberSubst(rt, cls).Apply(cls.Underlying.Type)
	case cls != nil && (!cls.External || c.kotlinSuperProp(cls, x.Name)):
		x.Prop = c.memberProp(cls, x.Name)
		if d := c.declaredClass(x.X, cls); x.Prop == nil && d != nil && !d.External {
			// A private property of a smart-cast variable's declared
			// type: a.p for a: A smart-cast to B, in A's code.
			if p := c.memberProp(d, x.Name); p != nil && cls.IsSubclassOf(p.Owner) {
				x.Prop = p
			}
		}
		if p := x.Prop; p != nil && protectedMember(p) {
			if d := c.declaredClass(x.X, cls); d != nil && p.Owner != d && !c.strictProtectedVia(p.Owner, cls) {
				// As in K2, the protected override of a smart-cast
				// variable's class that the code sees only as an override
				// of what it sees gives way to the declared type's
				// member: other.a of other: A smart-cast to B, in A's
				// code, is A's a, not B's override.
				if q := c.memberProp(d, x.Name); q != nil && cls.IsSubclassOf(q.Owner) {
					x.Prop = q
				}
			}
		}
		if x.Prop == nil {
			t, found = c.javaProperty(x, rt)
			c.protectedGetter(x)
			break
		}
		if p := x.Prop; !c.propVisible(p) || protectedMember(p) && !c.protectedVia(p.Owner, x.X) {
			// An extension of the name is read instead, as with calls.
			x.Prop = nil
			if t, found = c.extProp(x, rt); found {
				break
			}
			c.invisibleProp(x.P, p)
			x.Prop, found = p, true
		} else if ensureVar(p); p.PrivateToThis {
			c.checkPrivateToThis(x.X, x.Name, p.Owner, x.P)
		} else {
			c.shadowedField(cls, p, x.P)
			c.companionProtectedProp(p, x.P) // A.x of a companion's x
		}
		ensureVar(x.Prop)
		c.recursiveVarRef(x.Prop, x.P)
		t = c.memberSubst(rt, x.Prop.Owner).Apply(x.Prop.Type)
		t = c.intersectionProp(xt.NonNull(), rt, x.Prop, t)
		t = c.starBoundsProp(rt, x.Prop, t)
		if !x.Safe && c.stableOn(x.Prop, rt) {
			// A smart cast of a.b.
			x.T = t
			if k, ok := c.narrowKeyOf(x); ok && k.t != t {
				x.Cast = needsCast(t, k.t)
				t = k.t
			}
		}
		t = c.memberField(x, rt, t)
	case cls != nil && c.visibleField(instanceField(cls, x.Name)):
		x.Prop = instanceField(cls, x.Name)
		if x.Prop.Protected && !c.protectedVia(x.Prop.Owner, x.X) {
			c.invisibleProp(x.P, x.Prop)
		} else {
			c.shadowedField(cls, x.Prop, x.P)
		}
		t = c.memberSubst(rt, x.Prop.Owner).Apply(x.Prop.Type)
	default:
		t, found = c.javaProperty(x, rt)
		c.protectedGetter(x)
	}
	if x.Prop != nil && x.Prop.Ambiguous {
		c.errorf(x.P, "overload resolution ambiguity: %s", x.Name)
	}
	if found && x.Prop == nil && x.Getter != nil && !x.Safe {
		// A smart cast of a.b, for a friend path's stable property.
		x.T = t
		if k, ok := c.narrowKeyOf(x); ok && k.t != t {
			x.Cast = needsCast(t, k.t)
			t = k.t
		}
	}
	if !found {
		if t, found = c.extProp(x, rt); !found {
			c.errorf(x.P, "unresolved reference: %s", x.Name)
		}
	}
	if t.K == types.Invalid {
		return t
	}
	if x.Safe {
		x.SelNonNull = !types.MayBeNull(t)
		x.SafeT = t
		t = t.Nullable()
	}
	return t
}

// intersectionProp returns the type of property p, of type t seen
// through rt, read on a receiver of intersection type xt: the
// intersection of its types seen through each component of rt's class,
// as K2's intersection scope has it. value of a C<out T> & C<out V> is a
// T & V.
func (c *Checker) intersectionProp(xt, rt types.Type, p *types.VarSym, t types.Type) types.Type {
	if !xt.IsIntersection() || rt.K != types.Class || p.Owner == nil {
		return t
	}
	ts := []types.Type{t}
	for _, o := range xt.Param().Bounds {
		if o = o.NonNull(); o == rt || o.K != types.Class || o.C != rt.C {
			continue
		}
		u := c.memberSubst(o, p.Owner).Apply(p.Type)
		if u.K == types.Invalid || u.HasVars() {
			return t
		}
		ts = append(ts, u)
	}
	if len(ts) == 1 || t.HasVars() {
		return t
	}
	// The components as they are, T & V of two type parameters that may
	// be null, not (T & Any & V & Any)?: of equal ones the first stays.
	var comps []types.Type
	for i, u := range ts {
		if !slices.ContainsFunc(ts[:i], func(o types.Type) bool { return c.subtype(o, u) }) &&
			!slices.ContainsFunc(ts[i+1:], func(o types.Type) bool { return c.subtype(o, u) && !c.subtype(u, o) }) {
			comps = append(comps, u)
		}
	}
	if len(comps) == 1 {
		return comps[0]
	}
	if slices.ContainsFunc(comps, func(u types.Type) bool { return u.N || u.D || u.P || u.IsIntersection() }) {
		if r := c.intersect(comps); r.K != types.Invalid {
			return r
		}
		return t
	}
	return c.internIntersection(comps)
}

// kotlinSuperProp reports whether Java class cls inherits property name
// of a Kotlin class of the program: the property of JavaClass().some for
// a JavaClass extends Derived, Derived declaring or inheriting val some.
func (c *Checker) kotlinSuperProp(cls *types.ClassSym, name string) bool {
	if cls.KotlinName != "" || cls.Java != nil || cls.Value {
		return false
	}
	p := c.memberProp(cls, name)
	return p != nil && p.Owner != nil && !p.Owner.External
}

// effectless reports whether evaluating x has no effects to keep when
// its value isn't used: this or a name.
func effectless(x syntax.Expr) bool {
	switch x.(type) {
	case *syntax.This, *syntax.Ident:
		return true
	}
	return false
}

// invokableProp reports whether name is a property of this class or a
// top-level one that can be invoked: local functions that don't take a
// call give way to it, as context(a: String) fun c() to val c:
// context(String) () -> String in c("1").
func (c *Checker) invokableProp(name string) bool {
	if c.cls != nil {
		if p := c.userProp(c.cls, name); p != nil && !p.InProgress {
			ensureVar(p)
			return c.invokable(p.Type)
		}
	}
	v := c.topVar(name)
	return v != nil && !c.uninvokable(v, name)
}

// invokable reports whether a value of type t can be called: a function
// type, or a class with an invoke operator.
func (c *Checker) invokable(t types.Type) bool {
	t = t.Flex()
	switch {
	case t.K == types.Invalid || t.IsFunc():
		return true
	case t.K != types.Class:
		return false
	}
	return len(c.javaMethods(t.C, "invoke")) > 0 || len(c.extCallees("invoke", t)) > 0
}

func (c *Checker) call(x *syntax.Call, want types.Type, sink *system) types.Type {
	type mark struct{ cons, joined, extra int }
	var marks []mark
	amb := c.ambiguities
	for _, r := range c.pclaRuns {
		marks = append(marks, mark{len(r.cons), len(r.joined), len(r.extra)})
	}
	t := c.call1(x, want, sink)
	if id, ok := x.Fun.(*syntax.Ident); ok && id.Underscore && x.Ctor != nil {
		// _<Any>() of a class `_`, not the CHECK_TYPE helper.
		c.errorf(id.P, "names _, __, ___, ..., can be used only in back-ticks (`_`, `__`, `___`, ...)")
	}
	if c.ambiguities > amb {
		// An ambiguous call in a builder's lambda gives the builder's
		// variables nothing, as in kotlinc: what its candidates
		// recorded is dropped, what the calls before it did stays.
		for i, r := range c.pclaRuns[:len(marks)] {
			m := marks[i]
			if len(r.cons) < m.cons || len(r.joined) < m.joined || len(r.extra) < m.extra {
				continue
			}
			for _, v := range r.extra[m.extra:] {
				delete(c.stubs, v)
			}
			r.cons, r.joined, r.extra = r.cons[:m.cons], r.joined[:m.joined], r.extra[:m.extra]
		}
	}
	if len(c.pclaRuns) > 0 {
		t = c.pclaFailedCall(x, t)
	}
	if len(c.uninferred) > 0 {
		c.uninferredArg(x)
	}
	return t
}

func (c *Checker) call1(x *syntax.Call, want types.Type, sink *system) types.Type {
	if m, ok := x.Fun.(*syntax.Member); x.Recv || ok && (m.Paren || m.Grouped) {
		// (a?.b)(v) invokes the value of a?.b: the parentheses end
		// its chain. So does (a.b)(v), of a property b, not a function.
		return c.invoke(x, want, sink)
	}
	if id, ok := x.Fun.(*syntax.Ident); ok && id.Grouped {
		return c.invoke(x, want, sink) // (a)(v): the value of a, not a function a
	}
	if t, ok := c.safeChainInvoke(x, want, sink); ok {
		return t
	}
	switch fn := x.Fun.(type) {
	case *syntax.Ident:
		name := fn.Name
		if show := c.hideUnfitLocals(name, x); show != nil {
			defer show()
		}
		// A variable that can't be invoked doesn't hide functions, as in
		// fun tag(tag: Any?) = tag(Any::class, tag).
		_, t, isVar := c.scope.lookup(name)
		if _, depth := c.scope.find(name); isVar && c.invokable(t) && c.innerMemberTakes(name, x, depth) && c.recvCall(x, fn) {
			// A member of an implicit receiver nearer than the variable
			// comes first, as each level of scope in Kotlin: foo() in
			// with(A()) { } is A's foo, not an outer foo: A.() -> Int.
			return c.call(x, want, sink)
		}
		if isVar && c.invokable(t) {
			fits := c.localInvokeFits(t, x)
			if !fits && len(c.recvs) > 0 && c.recvCall(x, fn) {
				// One that can't take the arguments gives way to an
				// implicit receiver's function, as in Kotlin, where each
				// level of scope is tried in turn: map(crossinline
				// transform) = transform { value -> ... } calls Flow's
				// transform.
				return c.call(x, want, sink)
			}
			if fits || c.cls == nil || !c.anyMayTake(c.accessible(c.cls, c.javaMethods(c.cls, name)), x) {
				return c.invoke(x, want, sink)
			}
			// Or to a method of this class: a constructor parameter
			// log, with an invoke operator, and fun log() in an init block.
		}
		if fs, depth := c.scope.findFunsFor(name, func(fs []*types.FunSym) bool { return c.anyMayTake(fs, x) }); len(fs) > 0 && (c.anyMayTake(fs, x) || !c.invokableProp(name) && !c.anyMayTake(c.topFuns(name), x)) && !c.localGivesWay(fs, name, x) {
			// Local functions that can't take the arguments give way to
			// top-level ones that can, as each level in Kotlin.
			plain := slices.DeleteFunc(slices.Clone(fs), func(f *types.FunSym) bool { return f.Ext })
			switch {
			case len(plain) < len(fs) && len(plain) > 0 && c.anyMayTake(plain, x):
				// One without a receiver that takes the call comes before
				// the extensions: context(a: A) fun f() before fun A.f().
				fs = plain
			case len(plain) < len(fs):
				// A local extension function called on an implicit
				// receiver: add(1) in buildString { } is this.add(1).
				if len(c.recvs) > 0 && c.recvCall(x, fn) {
					return c.call(x, want, sink)
				}
				if c.cls != nil && c.thisExtension(name, x) {
					this := &syntax.This{Class: true}
					this.P = fn.P
					m := &syntax.Member{X: this, Name: name}
					m.P = fn.P
					x.Fun = m
					return c.call(x, want, sink)
				}
				if len(plain) == 0 && c.invokableProp(name) {
					return c.invoke(x, want, sink) // none has its receiver
				}
			}
			return c.callLocal(x, fs, depth, want, sink)
		}
		if len(c.recvs) > 0 && c.recvCall(x, fn) {
			// A function of an implicit receiver.
			if t, ok := c.onReceiver(x, fn, want, sink); ok {
				return t
			}
		}
		if c.cls != nil {
			// A method of this class, also inherited: f() is this.f(),
			// unless none takes the arguments and an object in scope's,
			// an imported member of an object, a top-level function of
			// the package or one of the class path's, or an extension of
			// this does.
			ms := c.lowPriorityGivesWay(c.accessible(c.cls, c.javaMethods(c.cls, name)), name, c.cls.This(), x)
			if !slices.ContainsFunc(ms, func(f *types.FunSym) bool { return !f.Hidden }) {
				// Hidden ones, @Deprecated(level = HIDDEN), give way to
				// the functions of the scopes outside.
				ms = nil
			}
			if len(ms) > 0 && (c.anyMayTake(ms, x) ||
				c.objectTaking(name, x) == nil && !c.memberImportTakes(name, x) && !c.anyMayTake(c.topFuns(name), x) && !c.anyMayTake(c.kotlinFuns(name, false), x) &&
					!c.thisExtension(name, x) && !c.propInvokeFits(name, x) && !c.ctorTakes(name, x)) &&
				// Members that don't take the arguments' types give way to
				// a property of the name whose value does: sut(state) next
				// to fun sut(a: A), for a state whose type is inferred. Its
				// type is inferred only then, as K2 infers it only if no
				// member is applicable: in declaration order otherwise.
				!(c.userProp(c.cls, name) != nil && !c.takeTypes(ms, x, c.argTypes(x)) && c.propInvokeFits(name, x)) {
				if c.thisExtension(name, x) && !c.takeTypes(ms, x, c.argTypes(x)) {
					// Members that don't take the arguments' types give way
					// to the extensions of this, as with this.f():
					// JobSupport's invokeOnCompletion(handler = node) is the
					// internal Job.invokeOnCompletion(handler: JobNode).
					this := &syntax.This{Class: true}
					this.P = fn.P
					m := &syntax.Member{X: this, Name: name}
					m.P = fn.P
					x.Fun = m
					return c.call(x, want, sink)
				}
				// Members that don't take the arguments' types give way to
				// top-level functions that do, the next level of scope:
				// take(s.ext) for an A, next to a member take(arg: B) and a
				// top-level take(arg: Any). The arguments are checked ahead
				// only where there are such functions.
				if fs := c.visible(c.topFunsFor(name, x), x); len(fs) > 0 && c.anyMayTake(fs, x) {
					if ts := c.argTypes(x); !c.takeTypes(ms, x, ts) && c.takeTypes(fs, x, ts) {
						return c.resolve(x, methodCallees(fs, nil), want, sink)
					}
				}
				c.captureThis()
				return c.resolve(x, c.memberCallees(ms, c.cls.This()), want, sink)
			}
			if p := c.userProp(c.cls, name); p != nil && !(inferring(p) && (c.namesCallee(name) || c.thisExtension(name, x))) {
				// One being inferred gives way to a function of its name:
				// val navOptions = navOptions { }.
				ensureVar(p)
				if c.invokable(p.Type) {
					return c.invoke(x, want, sink)
				}
			} else if p != nil {
				c.inferSyntheticGetter(c.cls, name)
			}
			if c.thisExtension(name, x) {
				// An extension function called on this: apply { } is
				// this.apply { }. As in kotlinc, it comes before the
				// functions of the objects in scope.
				this := &syntax.This{Class: true}
				this.P = fn.P
				m := &syntax.Member{X: this, Name: name}
				m.P = fn.P
				x.Fun = m
				if t, ok := c.onReceiver(x, fn, want, sink); ok {
					return t
				}
			}
			if cls := c.staticInScope(name); cls != nil {
				// A static method of a class the code is in: valueOf in an
				// enum.
				id := &syntax.Ident{Name: cls.DisplayName()}
				id.P = fn.P
				m := &syntax.Member{X: id, Name: name}
				m.P = fn.P
				x.Fun = m
				return c.staticCall(x, m, cls, want, sink)
			}
			if obj := c.objectMethods(name, x); obj != nil && (c.objectTaking(name, x) != nil || !c.importTakes(name, x)) {
				if cls := c.findClass(name); cls != nil && cls.Outer == obj && c.ctorsTake(cls, x) {
					// Or a class nested in it, whose constructors come
					// first, as the classes in scope do before the
					// functions of an implicit receiver in Kotlin: Bar("4")
					// in Owner, for a companion's class Bar(String?) and
					// fun Bar(String).
					return c.construct(x, cls, fn.P, want, sink)
				}
				// A method of an object in scope: f() is Companion.f().
				m := &syntax.Member{X: objectIdent(obj, fn.P), Name: name}
				m.P = fn.P
				x.Fun = m
				return c.call(x, want, sink)
			}
			if obj := c.objectExtension(name, x); obj != nil {
				// An extension of an object in scope, the companion of a
				// superclass too: foo() is A.Companion.foo(), for a
				// fun A.Companion.foo().
				m := &syntax.Member{X: objectIdent(obj, fn.P), Name: name}
				m.P = fn.P
				x.Fun = m
				return c.call(x, want, sink)
			}
			if c.thisMethods(name) {
				// A method of this's smart-cast type: f() is this.f().
				this := &syntax.This{Class: true}
				this.P = fn.P
				m := &syntax.Member{X: this, Name: name}
				m.P = fn.P
				x.Fun = m
				return c.call(x, want, sink)
			}
		}
		if c.cls == nil && c.staticCls != nil {
			// A companion's method in a this(...) call's arguments.
			if obj := c.objectMethods(name, x); obj != nil {
				m := &syntax.Member{X: objectIdent(obj, fn.P), Name: name}
				m.P = fn.P
				x.Fun = m
				return c.call(x, want, sink)
			}
		}
		if mi, ok := c.callableImport(name); ok && !(!c.memberImportTakes(name, x) && c.ctorTakes(name, x)) {
			// An imported static method or method of an object: f() is C.f().
			// Not where only the constructor of a class imported with
			// it takes the arguments: import a.S.A of class A and fun A(c).
			m := c.importedRef(mi, fn.P)
			x.Fun = m
			if m.X.(*syntax.Ident).Object != nil {
				return c.call(x, want, sink)
			}
			return c.staticCall(x, m, mi.owner, want, sink)
		}
		if fs := c.importShadowed(name, x); len(fs) > 0 {
			return c.resolve(x, methodCallees(fs, nil), want, sink)
		}
		if fs := c.visible(c.topFunsFor(name, x), x); len(fs) > 0 {
			if cls := c.findClass(name); c.ctorBeatsFuns(cls, fs, x) {
				return c.construct(x, cls, fn.P, want, sink)
			} else if c.starCtorTiesFuns(name, cls, fs, x) {
				c.errorf(fn.P, "overload resolution ambiguity: %s is a constructor and a function of the star imports", name)
			}
			// The program's functions that don't fit the call give way to
			// the class path's, as in Kotlin, where each level of scope is
			// tried in turn: okio's minOf(Long, Int) to the standard
			// library's minOf(Long, Long).
			if !c.libraryCallable(name) {
				// Or to a library class of the same name, imported with
				// them: Items() next to a program's Items(range).
				if cls := c.findClass(name); cls != nil && cls.External && constructible(cls) && c.ctorsFit(cls, fs, x) {
					return c.construct(x, cls, fn.P, want, sink)
				}
				return c.resolve(x, methodCallees(fs, nil), want, sink)
			}
			fit := c.anyMayTake(fs, x)
			if fit && hasLambda(x.Args) {
				return c.resolve(x, methodCallees(fs, nil), want, sink)
			}
			if fit {
				// Resolving checks the arguments; lambdas are checked once.
				n := len(c.errs)
				if t := c.resolve(x, methodCallees(fs, nil), want, sink); len(c.errs) == n {
					return t
				}
				c.errs = c.errs[:n]
			}
		}
		if v := c.topVar(name); v != nil && !(c.libraryVar(v) && len(c.kotlinFuns(name, false)) > 0) && !c.uninvokable(v, name) {
			// A property of function type, invoked; a library's function
			// of the same name comes first: EmptySerializersModule().
			return c.invoke(x, want, sink)
		}
		if c.valueImport(name) {
			// An imported property or enum entry, invoked: import A.ONE,
			// then ONE(42) is A.ONE.invoke(42).
			return c.invoke(x, want, sink)
		}
		if name == "String" && len(x.Args) == 0 || len(x.Args) == 0 && c.aliasOfString(name) {
			x.Builtin = types.NewString // String(): a new empty string
			return types.StringType
		}
		if f := arrayFuns[name]; f != nil && (name != "String" || len(c.kotlinFuns(name, false)) == 0 ||
			c.mayTake(f, x) && c.takeTypes([]*types.FunSym{f}, x, c.argTypes(x))) {
			// String(chars); the standard library has String(bytes),
			// String(bytes, charset) and others. mayTake can't see the
			// type of an argument that is a call, as in
			// String(s.toByteArray()), so the arguments are checked first.
			return c.arrayCall(x, f, want, sink)
		}
		if b, ok := assertions[name]; ok && len(c.funImports[name]) == 0 {
			return c.assertCall(x, b, name, want)
		}
		if name == "kc$coerce" && c.state.plugin {
			return c.coerceCall(x)
		}
		alias := c.findClassifier(name).alias
		cls := c.findClass(name)
		if k := c.packageClassifierPastBounds(name, x); k != (classifier{}) {
			alias, cls = k.alias, k.cls
			if alias != nil {
				cls = c.aliasClass(alias)
			}
		}
		if alt := c.packageClassTaking(name, cls, x); alt != nil {
			cls = alt
		}
		if name == "Array" && cls != nil && cls != types.ArrayClass && !cls.External && !c.ctorsTake(cls, x) {
			cls = types.ArrayClass // kotlin.Array(n) { }, past a class Array of the program's
		}
		var fs []*types.FunSym
		if cls == nil || !constructible(cls) {
			// A function may have a class's name: kotlin.random.Random(seed).
			fs = c.kotlinFuns(name, false)
		} else if gs := c.kotlinFuns(name, false); len(gs) > 0 && cls.External {
			// Or one of the class's: kotlinx.coroutines'
			// CancellationException(message, cause) for the typealias of
			// a class without that constructor.
			if !c.ctorsFit(cls, gs, x) {
				fs = gs
			}
		}
		if (cls != nil || alias != nil) && len(fs) == 0 && c.ambiguousCtor(name) {
			// import a.A.B, import a.D.B: B() is unresolved, as K2's.
			c.errorf(fn.P, "unresolved reference: %s", name)
			c.args(x, nil)
			return types.InvalidType
		}
		if a := alias; a != nil && cls != nil && !cls.Object && len(fs) == 0 {
			c.recordAlias(x, a)
			return c.aliasConstruct(x, a, cls, fn.P, want, sink)
		}
		if cls != nil && cls.Object && len(fs) == 0 && c.hasFun(types.ClassType(cls), "invoke", x) {
			// An object with an invoke operator: Colors() is
			// Colors.invoke().
			x.Fun = objectIdent(cls, fn.P)
			return c.invokeValue(x, types.ClassType(cls), want, sink)
		}
		if cls != nil && len(fs) == 0 {
			if alias == nil && c.libraryAliasOf(name, cls) {
				if c.libAliasCtors == nil {
					c.libAliasCtors = map[*syntax.Call]bool{}
				}
				c.libAliasCtors[x] = true
			}
			return c.construct(x, cls, fn.P, want, sink)
		}
		if a := c.findClassifier(name).alias; a != nil && cls == nil && len(fs) == 0 && len(a.TypeParams) == 0 {
			// ktor's actual typealias SynchronizedObject = Any.
			c.resolveAlias(a)
			if a.Type == types.AnyType {
				name = "Any"
			}
		}
		if name == "Any" && len(fs) == 0 {
			// Any(): a java.lang.Object, typed Any.
			if object := c.u.ext.class("java/lang/Object"); object != nil {
				if c.construct(x, object, fn.P, want, sink).K == types.Invalid {
					return types.InvalidType
				}
				return types.AnyType
			}
		}
		if len(fs) > 0 {
			t := c.resolve(x, methodCallees(fs, nil), want, sink)
			if x.Sym != nil {
				x.Builtin = x.Sym.Builtin
			}
			return t
		}
		if b, ok := builtins[name]; ok && !c.u.ext.kotlin {
			c.noSpread(x)
			x.Sym, x.Builtin = b, b.Builtin
			if len(x.TypeArgs) > 0 {
				c.errorf(x.TypeArgs[0].P, "no type arguments expected for '%s'", name)
			}
			if len(x.Args) > 1 || b.Builtin == types.Print && len(x.Args) != 1 {
				c.errorf(x.P, "wrong number of arguments for %s", name)
			}
			for _, a := range x.Args {
				if c.value(a, noWant).K == types.Unit {
					c.errorf(a.Pos(), "printing Unit is not supported")
				}
			}
			return types.UnitType
		}
		if isVar || c.cls != nil && c.userProp(c.cls, name) != nil {
			return c.invoke(x, want, sink) // reports that it can't be invoked
		}
		if (c.cls != nil || c.staticCls != nil) && c.objectHasProp(name) {
			// A property of an object in scope, invoked: an enclosing
			// class's companion's val f: () -> T, as f().
			return c.invoke(x, want, sink)
		}
		if c.cls != nil {
			if hidden := c.javaMethods(c.cls, name); len(hidden) > 0 {
				// Only methods of this that the code can't see: a
				// superclass's private ones.
				c.invisibleFun(fn.P, hidden[0])
				c.args(x, nil)
				return types.InvalidType
			}
		}
		if rt, ok := c.nullableImplicit(name, x); ok {
			c.errorf(fn.P, "only safe (?.) or non-null asserted (!!.) calls are allowed on a nullable receiver of type %s", rt)
			c.args(x, nil)
			return c.unsafeCallResult(x, rt, name)
		}
		if t, ok := c.inaccessibleOuterCall(fn.P, name, nil); ok {
			c.args(x, nil)
			return t
		}
		c.errorf(fn.P, "unresolved reference: %s", name)
		c.args(x, nil)
		return types.InvalidType

	case *syntax.Member:
		if id, ok := fn.X.(*syntax.Ident); ok && fn.Static != nil && id.Object == nil {
			// A static call checked before, as argTypes checks arguments:
			// f() of import p.J.f is J.f(), and J may not be in scope.
			return c.staticCall(x, fn, fn.Static, want, sink)
		}
		if fn.Safe && c.qualifierValue(fn.X) {
			c.noteQualifierSafe(fn)
			c.args(x, nil)
			return types.InvalidType
		}
		pkg := c.packageRef(fn.X)
		if pkg != nil {
			// A top-level function by its qualified name: a.b.f(). A class
			// of the name whose constructors may take the call, and none of
			// the functions, is constructed: the serialization plugin's
			// io.ktor.util.date.GMTDate(seen0, ...) next to ktor's
			// functions GMTDate(timestamp) and GMTDate(seconds, ...).
			if fs := nonExt(pkg.Funs[fn.Name]); len(fs) > 0 && !c.ctorsTakeOnly(c.classRef(fn), fs, x) {
				return c.resolve(x, methodCallees(fs, nil), want, sink)
			}
		} else if fs := c.libraryPackageFuns(fn); len(fs) > 0 && !c.ctorsTakeOnly(c.classRef(fn), fs, x) {
			return c.resolve(x, methodCallees(fs, nil), want, sink) // kotlin.runCatching { }
		} else if c.kotlinIntrinsic(fn) {
			// kotlin.arrayOf(x), which arrayCall checks by its simple name.
			id := &syntax.Ident{Name: fn.Name}
			id.P = fn.P
			x.Fun = id
			return c.call(x, want, sink)
		}
		if a := c.qualifiedAlias(fn); a != nil && len(x.Args) == 0 && c.stringAlias(a) {
			id := &syntax.Ident{Name: "String"} // Class.S(), S a typealias of String
			id.P = fn.P
			x.Fun, x.Builtin = id, types.NewString
			if c.aliasCtors == nil {
				c.aliasCtors = map[*syntax.Call]*types.AliasSym{}
			}
			c.aliasCtors[x] = a // for its deprecation
			return types.StringType
		}
		if cls := c.classRef(fn); cls != nil {
			if name := c.qualifiedName(fn); name != "" {
				if a := c.lookupAlias(name); a != nil {
					c.recordAlias(x, a)
					return c.aliasConstruct(x, a, cls, fn.P, want, sink) // a.b.Alias(1)
				}
			}
			if name := c.qualifiedName(fn); name != "" && c.libraryAliasPath(name, cls) {
				if c.libAliasCtors == nil {
					c.libAliasCtors = map[*syntax.Call]bool{}
				}
				c.libAliasCtors[x] = true // kotlin.collections.HashSet(c)
			}
			return c.construct(x, cls, fn.P, want, sink) // java.util.Random(1)
		}
		if pkg != nil {
			c.errorf(fn.P, "unresolved reference: %s", fn.Name)
			c.args(x, nil)
			return types.InvalidType
		}
		if cls := c.classRef(fn.X); cls != nil && (objectOf(cls) == nil || len(javaStatics(cls, fn.Name)) > 0) {
			return c.staticCall(x, fn, cls, want, sink)
		}
		c.rootObject(fn.X)
		if s, ok := fn.X.(*syntax.Super); ok {
			c.superSafeCall(fn)
			return c.superCall(x, fn, s, want, sink)
		}
		if _, ok := c.extFuncVar(fn.Name); ok && fn.Safe && !c.memberExtFirst(fn, x) {
			// x?.f() calls a function value with a receiver if x isn't
			// null: x?.let { r -> f(r) }, which evaluates the arguments
			// only then. ktor's reinterpret?.block(subject).
			r := &syntax.Ident{Name: "$receiver"}
			r.P = fn.X.Pos()
			id := &syntax.Ident{Name: fn.Name}
			id.P = fn.P
			inner := &syntax.Call{Fun: id, Args: append([]syntax.Expr{r}, x.Args...)}
			inner.P = x.P
			if x.Names != nil {
				inner.Names = append([]string{""}, x.Names...)
			}
			lam := &syntax.Lambda{Arrow: true, Params: []*syntax.LambdaParam{{P: r.P, Name: r.Name}},
				Body: &syntax.Block{P: x.P, Stmts: []syntax.Stmt{&syntax.ExprStmt{X: inner}}}}
			lam.P = x.P
			let := &syntax.Member{X: fn.X, Name: "let", Safe: true}
			let.P = fn.P
			x.Fun, x.Args, x.Names = let, []syntax.Expr{lam}, nil
			return c.call(x, want, sink)
		}
		ft, ok := c.extFuncVar(fn.Name)
		if p := c.orderDependent; p != nil && p.Owner != nil {
			c.warnf(fn.P, "IMPLICIT_PROPERTY_TYPE_MAKES_BEHAVIOR_ORDER_DEPENDANT",
				"the resolution result depends on the declaration order. This can lead to false-positive errors in the IDE. Specify the type of '%s' defined in '%s' explicitly", p.Name, p.Owner.DisplayName())
		}
		if (ok || c.extRefLocal(fn.Name)) && !fn.Safe && !c.memberExtFirst(fn, x) {
			// x.f() calls a function value with a receiver: f(x).
			if rt := ft.Flex().NonNull(); ok && rt.Contexts() > 0 && len(x.Args)+1 == rt.A.Len()-1 {
				// With an argument for each context parameter, x among
				// them, which K2 doesn't support: "".f(1, true) for an
				// f: context(String, Int) Boolean.() -> Unit.
				c.errorf(fn.P, "unsupported contextual declaration call: '%s' takes its context arguments implicitly", fn.Name)
			}
			id := &syntax.Ident{Name: fn.Name}
			id.P = fn.P
			x.Fun, x.Args = id, append([]syntax.Expr{fn.X}, x.Args...)
			return c.invoke(x, want, sink)
		}
		rw := noWant
		if intLiteralOps[fn.Name] && want.Flex().NonNull() == types.LongType && literalArith(fn.X) && !slices.ContainsFunc(x.Args, func(a syntax.Expr) bool { return !literalArith(a) }) {
			// As 1 + 2, 1 shl 31, 1.plus(2) and (2 + 2 * 3).inv() where
			// a Long is expected are a Long's. Not 1.inc().
			rw = types.LongType
		}
		rt, nullExts, ok := c.callReceiver(fn, rw)
		if !ok {
			c.args(x, nil)
			return c.unsafeCallResult(x, rt, fn.Name)
		}
		n := c.safeReceivers(x)
		if r, ok := fn.X.(*syntax.Call); ok && !fn.Safe {
			// The arguments are evaluated once the receiver's call has
			// returned: requireNotNull(x).let { x.f() }.
			n = append(n, c.returnNarrows(r)...)
		}
		if len(n) > 0 || fn.Safe {
			// The arguments are evaluated only if the receiver isn't
			// null: x?.let { f(x) }, whose smart casts don't outlive
			// it: a?.f(y!!).
			c.push()
			c.scope.narrows = n
			defer func() {
				c.pop()
				// The vars its lambdas assign lose their smart casts
				// past it too (see checkLambda1), which may not run.
				var lambdas []any
				for _, a := range x.Args {
					if l, ok := a.(*syntax.Lambda); ok {
						lambdas = append(lambdas, l.Body)
					}
				}
				c.reset(c.watchedVars(lambdas...))
			}()
		}
		if fn.Safe {
			// x?.f() is f's result or null: an expected T? expects f's
			// result F to be a T?, as F? <: T? does in Kotlin, so that a
			// generic f whose result is nullable infers from it:
			// x?.let { res(it) }.
			if want.K == types.Invalid || !types.AcceptsNull(want) || want.HasVars() {
				want = noWant
			}
		}
		if types.MayBeNull(rt) && !rt.P && !fn.Safe && nullExts == nil && fn.Name == "plus" {
			// String?.plus(Any?): "null" for null, as s + x.
			if len(x.Args) != 1 || x.Names != nil {
				c.errorf(x.P, "wrong number of arguments for %s", fn.Name)
				c.args(x, nil)
				return types.InvalidType
			}
			if c.value(x.Args[0], types.AnyType.Nullable()).K == types.Invalid {
				return types.InvalidType
			}
			x.Builtin = types.StrPlus
			fn.T = rt
			return types.StringType
		}
		if types.MayBeNull(rt) && !rt.P && !fn.Safe && nullExts == nil {
			// Any?.toString(): String.valueOf(x), "null" for null.
			if len(x.Args) != 0 {
				c.errorf(x.P, "wrong number of arguments for %s", fn.Name)
				c.args(x, nil)
			}
			x.Builtin = types.ToString
			fn.T = rt
			return types.StringType
		}
		var t types.Type
		rt = c.callBound(rt, x, fn.Name)
		cls := c.classOf(rt)
		if cls != nil {
			n, alias := c.innerNamed(cls, fn.Name)
			if n != nil && fn.Safe {
				// outer?.Inner() constructs, and evaluates the arguments,
				// only if outer isn't null: outer?.let { r -> r.Inner() }.
				r := &syntax.Ident{Name: "$receiver"}
				r.P = fn.X.Pos()
				m := &syntax.Member{X: r, Name: fn.Name}
				m.P = fn.P
				inner := &syntax.Call{Fun: m, Args: x.Args, Names: x.Names}
				inner.P = x.P
				lam := &syntax.Lambda{Arrow: true, Params: []*syntax.LambdaParam{{P: r.P, Name: r.Name}},
					Body: &syntax.Block{P: x.P, Stmts: []syntax.Stmt{&syntax.ExprStmt{X: inner}}}}
				lam.P = x.P
				let := &syntax.Member{X: checked(fn.X, fn.X.Type()), Name: "let", Safe: true}
				let.P = fn.P
				x.Fun, x.Args, x.Names = let, []syntax.Expr{lam}, nil
				return c.call(x, want, sink)
			} else if n != nil {
				fn.T = rt
				if alias != nil && !c.aliasOuterFits(alias, n, rt) {
					// x.GIntI() for typealias GIntI = Generic<Int>.Inner
					// needs an x that is a Generic<Int>.
					c.errorf(fn.P, "unresolved reference: %s, which needs a receiver of type %s", fn.Name, c.aliasOuter(alias, n))
					c.args(x, nil)
					return types.InvalidType
				}
				if alias != nil && hasUnderscore(x.TypeArgs) {
					// outer.Alias<_>(): the class's, which the call infers.
					if !c.aliasUnderscores(x, alias, n, fn.P) {
						c.args(x, nil)
						return types.InvalidType
					}
				} else if alias != nil && len(x.TypeArgs) > 0 {
					// outer.Alias<A>(): the alias's type arguments.
					if want = c.expandAlias(alias, &syntax.TypeRef{P: fn.P, Name: alias.Name, Args: x.TypeArgs}); want.K == types.Invalid {
						c.args(x, nil)
						return want
					}
					x.TypeArgs = nil
					if !want.HasVars() {
						sink = nil // complete, as in aliasConstruct
					}
				}
				return c.innerConstruct(x, n, fn.X, rt, fn.P, want, sink) // outer.Inner()
			}
		}
		if nullExts == nil && !fn.Safe && c.extPropInvoked(fn, rt, cls, x) {
			// An extension property of function type, called: x.f(y).
			fn.X = checked(fn.X, rt)
			fn.T = c.member(fn)
			if g := fn.Getter; g != nil && g.Ext {
				// Its receivers, implicit, as a read of it takes them.
				c.dslMember(fn, fn, true, g.Owner, fn.Name, func() bool { return fn.Getter == g })
			}
			return c.invokeValue(x, fn.T, want, sink)
		}
		switch {
		case nullExts != nil:
			t = c.resolve(x, nullExts, want, sink)
		case cls != nil && !cls.External:
			ms := c.javaMethods(cls, fn.Name)
			if p := c.userProp(cls, fn.Name); len(ms) == 0 && p != nil && !((!c.propVisible(p) || p.InProgress || !c.invokeFits(p, x)) && c.extCallees(fn.Name, rt) != nil) {
				// A property of function type, called: obj.f(x).
				if fn.Safe {
					return c.safeInvoke(x, fn, want, sink)
				}
				ensureVar(p)
				fn.Prop = p
				if !c.propVisible(p) {
					c.invisibleProp(fn.P, p)
				}
				fn.T = c.memberSubst(rt, p.Owner).Apply(p.Type)
				return c.invokeValue(x, fn.T, want, sink)
			}
			t = c.javaCall(x, fn, rt, want, sink)
		case cls != nil && len(c.javaMethods(cls, fn.Name)) == 0 && c.extCallees(fn.Name, rt) == nil && c.hasProp(cls, fn.Name):
			// A property of the class path, called: d.onAwait { } in a
			// select, whose value an invoke operator takes.
			if fn.Safe {
				return c.safeInvoke(x, fn, want, sink)
			}
			pt, _ := c.javaProperty(fn, rt)
			fn.T = pt
			return c.invokeValue(x, fn.T, want, sink)
		case cls != nil:
			t = c.javaCall(x, fn, rt, want, sink)
		default:
			t = c.builtinMember(x, fn, rt, want, sink)
		}
		if x.Sym != nil {
			x.Builtin = x.Sym.Builtin
			if x.Infix && !x.Sym.Infix || x.Operator && !x.Sym.Operator {
				if c.noneApplicable(x, fn, rt) {
					// As K2, none of several candidates the call may be
					// is applicable: its type is an error, which a
					// lambda's result can't infer a type with.
					t = types.InvalidType
				}
			}
			if x.Infix && !x.Sym.Infix {
				c.errorf(fn.P, "'infix' modifier is required on '%s'", fn.Name)
			}
			if x.Operator && !x.Sym.Operator {
				c.errorf(x.P, "'operator' modifier is required on '%s'", fn.Name)
			} else if x.Operator && renamedOperator(x.Sym, fn.Name) {
				c.errorf(x.P, "'operator' modifier is required on '%s': operator '%s' is imported as '%s'", fn.Name, x.Sym.Name, fn.Name)
			}
			if isArrayFactory(x.Builtin) {
				c.reified(x, t, sink)
			}
		}
		if fn.Safe {
			fn.SafeT = t
		}
		if fn.Safe && t.K != types.Unit {
			fn.Nothing = t.K == types.Nothing && !t.N
			fn.SelNonNull = !types.MayBeNull(t)
			t = t.Nullable()
		}
		fn.T = rt
		return t
	}
	return c.invoke(x, want, sink)
}

// noneApplicable reports whether infix or operator call x of a local
// extension lacking the modifier has a top-level extension of the name
// that may take it too. K2 then reports that none of them is applicable,
// rather than resolving x to the local one.
func (c *Checker) noneApplicable(x *syntax.Call, fn *syntax.Member, rt types.Type) bool {
	if _, local := x.Sym.Decl.(*syntax.LocalFun); !local || !x.Sym.Ext {
		return false
	}
	return slices.ContainsFunc(slices.Concat(c.funsOf(c.pkg, fn.Name), c.userFuns[fn.Name]), func(f *types.FunSym) bool {
		return f.Ext && f != x.Sym && c.recvFits(rt, f.Recv) && c.mayTake(f, x)
	})
}

// kotlinIntrinsic reports whether fn, kotlin.f in a call, names an array
// function of the kotlin package that the program doesn't declare.
func (c *Checker) kotlinIntrinsic(fn *syntax.Member) bool {
	if arrayFuns[fn.Name] == nil || !c.u.ext.kotlin || c.qualifiedName(fn.X) != "kotlin" || c.findClass("kotlin") != nil {
		return false
	}
	fs, _ := c.scope.findFuns(fn.Name)
	return len(fs) == 0 && len(c.topFuns(fn.Name)) == 0
}

// extPropInvoked reports whether call x of fn, x.f(args) on a receiver of
// type rt and class cls, invokes an extension property's value: the
// receiver has no member or extension function f and no member property
// f, and an extension property f whose value may take the arguments.
func (c *Checker) extPropInvoked(fn *syntax.Member, rt types.Type, cls *types.ClassSym, x *syntax.Call) bool {
	if cls != nil && c.hasProp(cls, fn.Name) || c.hasFun(rt, fn.Name, x) {
		return false
	}
	probe := &syntax.Member{X: checked(fn.X, rt), Name: fn.Name}
	probe.P = fn.P
	n := len(c.errs)
	pt, ok := c.extProp(probe, rt)
	c.errs = c.errs[:n]
	return ok && c.invokableWith(pt, x)
}

// uninvokable reports whether top-level property v, named name, can't be
// invoked while a class or function of the name can be called: in Kotlin
// such a property isn't a candidate, so val Border = 1.dp doesn't hide
// the constructor Border(width, color).
func (c *Checker) uninvokable(v *types.VarSym, name string) bool {
	other := c.namesCallee(name)
	if v.InProgress {
		// Its own initializer calls the class or function: val Shapes =
		// Shapes(...).
		return other
	}
	ensureVar(v)
	return !c.invokable(v.Type) && other
}

// ctorsTakeOnly reports whether a class's constructors may take call x and
// none of functions fs, of the same name, may, or one is more specific
// than those that may (see ctorBeatsFuns).
func (c *Checker) ctorsTakeOnly(cls *types.ClassSym, fs []*types.FunSym, x *syntax.Call) bool {
	if c.ctorBeatsFuns(cls, fs, x) {
		return true // or are more specific: bar.Foo("2") for Foo(String) and Foo(String?)
	}
	if cls == nil || !constructible(cls) || c.anyMayTake(fs, x) {
		return false
	}
	cls.Ensure()
	return c.anyMayTake(cls.Ctors, x)
}

// importShadowed returns the functions of the file's package with a name
// that an explicit import of the class path hides, if none of the
// import's functions and constructors may take call x and one of the
// package's may: as in Kotlin, the next level of scope is tried. ktor's
// ByteString(ByteBuffer) is called next to an import of kotlinx.io's
// ByteString, a class and a ByteString(vararg Byte).
func (c *Checker) importShadowed(name string, x *syntax.Call) []*types.FunSym {
	gs := c.funImports[name]
	if len(gs) == 0 || len(c.userFuns[name]) > 0 {
		return nil
	}
	fs := nonExt(c.pkg.Funs[name])
	if !c.anyMayTake(fs, x) || c.anyMayTake(nonExt(gs), x) {
		return nil
	}
	if cls := c.imports[name]; cls != nil && constructible(cls) {
		cls.Ensure()
		if c.anyMayTake(cls.Ctors, x) {
			return nil
		}
	}
	return fs
}

// invokeFits reports whether a property's value may be invoked with a
// call's arguments, or if it isn't of function type, whose invoke is
// resolved. A call with named arguments, or of another number of them,
// is an extension function's of the property's name, as in Kotlin, where
// inapplicable candidates give way: clikt's ProcessedArgument has a
// transformAll property and a transformAll(nvalues, required) extension.
func (c *Checker) invokeFits(p *types.VarSym, x *syntax.Call) bool {
	ensureVar(p)
	return c.invokeFitsType(p.Type, x)
}

// localInvokeFits is invokeFitsType for a local variable's type, whose type
// parameters are those of the declarations it is in: a lambda isn't one.
func (c *Checker) localInvokeFits(t types.Type, x *syntax.Call) bool {
	if !c.invokeFitsType(t, x) {
		return false
	}
	t = t.Flex()
	if !t.IsFunc() {
		return true
	}
	params := t.A.List[:t.A.Len()-1]
	params = params[len(params)-len(x.Args):] // without an implicit receiver's
	for i, a := range x.Args {
		if _, ok := a.(*syntax.Lambda); ok && params[i].T.K == types.TypeParam {
			return false
		}
	}
	return true
}

// propInvokeFits reports whether this class has a property named name
// whose value may be invoked with call x's arguments, which comes before
// its methods named so that can't take them: a property log, with an
// invoke operator, and fun log().
func (c *Checker) propInvokeFits(name string, x *syntax.Call) bool {
	p := c.userProp(c.cls, name)
	if p == nil || p.InProgress {
		return false
	}
	ensureVar(p)
	return c.invokable(p.Type) && c.localInvokeFits(p.Type, x)
}

// packageClassTaking returns the class named name of the code's package,
// if cls, an imported class of that name, has no constructor that may
// take call x's arguments and it has: as in Kotlin, a call that the
// explicitly imported class can't take goes to the next level of scope.
func (c *Checker) packageClassTaking(name string, cls *types.ClassSym, x *syntax.Call) *types.ClassSym {
	if cls == nil || c.imports[name] != cls {
		return nil
	}
	alt := c.pkg.Classes[name]
	if alt == nil {
		path := name
		if c.file.Package != "" {
			path = strings.ReplaceAll(c.file.Package, ".", "/") + "/" + name
		}
		alt = c.u.ext.class(path)
	}
	if alt == nil || alt == cls || !constructible(alt) {
		return nil
	}
	cls.Ensure()
	alt.Ensure()
	if !c.anyMayTake(userCtors(alt), x) || hasLambda(x.Args) {
		return nil
	}
	if c.anyMayTake(userCtors(cls), x) {
		ts := c.argTypes(x)
		if c.takeTypes(userCtors(cls), x, ts) || !c.takeTypes(userCtors(alt), x, ts) {
			return nil
		}
	}
	return alt
}

// packageClassifierPastBounds returns the class or typealias named name of
// the code's package when name is an explicitly imported typealias whose
// expansion, with call x's type arguments, gives its class a type argument
// out of its bounds. As in Kotlin 2.4
// (ProperlyCheckUpperBoundsViolationsWhenCreatingFreshVariables), that
// constructor isn't a candidate, and the call goes to the next level of
// scope: Alias<Any>() is b.Alias's with import a.Alias, typealias Alias<T>
// = Base<List<T>> for class Base<K : List<CharSequence>>, in package b
// with typealias Alias<K> = Box<K>.
func (c *Checker) packageClassifierPastBounds(name string, x *syntax.Call) classifier {
	a, ok := c.aliasImports[name]
	if !ok || len(x.TypeArgs) == 0 || len(x.TypeArgs) != len(a.TypeParams) || hasUnderscore(x.TypeArgs) || c.findClassifier(name).alias != a {
		return classifier{}
	}
	if c.renamedAway(c.pkg.Name, name) {
		return classifier{}
	}
	var alt classifier
	if cls, ok := c.pkg.Classes[name]; ok {
		alt = classifier{cls: cls}
	} else if al, ok := c.pkg.Aliases[name]; ok && al != a {
		alt = classifier{alias: al}
	}
	if alt == (classifier{}) || c.privateElsewhere(alt) || !c.expansionOutOfBounds(a, x.TypeArgs) {
		return classifier{}
	}
	return alt
}

// expansionOutOfBounds reports whether the expansion of typealias a with
// the type arguments args gives its class a type argument that isn't a
// subtype of the bound of the class's type parameter. It reports no
// errors.
func (c *Checker) expansionOutOfBounds(a *types.AliasSym, args []*syntax.TypeArg) bool {
	c.resolveAlias(a)
	if a.Type.K != types.Class {
		return false
	}
	n := len(c.errs)
	defer func() { c.errs = c.errs[:n] }()
	targs := make([]types.Arg, len(args))
	for i, r := range args {
		targs[i] = types.Arg{V: r.Variance}
		if r.Variance == types.Star {
			continue
		}
		if r.Type == nil {
			return false
		}
		if targs[i].T = c.resolveType(r.Type); targs[i].T.K == types.Invalid {
			return false
		}
	}
	t := a.Expand(targs)
	if t.K != types.Class || t.A == nil || len(t.C.TypeParams) != len(t.A.List) {
		return false
	}
	s := classSubst(t)
	for i, arg := range t.A.List {
		if arg.V == types.Star || arg.T.K == types.Invalid || arg.T.Has(func(u types.Type) bool { return u.K == types.TypeVar }) {
			continue
		}
		for _, b := range t.C.TypeParams[i].Bounds {
			if !c.subtype(arg.T, s.Apply(b)) {
				return true
			}
		}
	}
	return false
}

// memberImportTakes reports whether a member of an object that the file
// imports by name, as import p.Utils.group, may take call x's arguments.
func (c *Checker) memberImportTakes(name string, x *syntax.Call) bool {
	for _, mi := range c.memberImportsOf(name) {
		if c.anyMayTake(c.javaMethods(mi.owner, mi.name), x) {
			return true
		}
	}
	return false
}

// importTakes reports whether a function that the file imports, or one
// of the package or the class path, may take call x's arguments, which
// the methods of the objects in scope named so give way to when none of
// them can.
func (c *Checker) importTakes(name string, x *syntax.Call) bool {
	return c.memberImportTakes(name, x) || c.anyMayTake(c.topFuns(name), x) || c.anyMayTake(c.kotlinFuns(name, false), x)
}

// ctorTakes reports whether a class named name has a constructor that
// may take call x's arguments, which comes after this class's methods of
// the name that can't: Settings(mode = 1) next to fun Settings().
func (c *Checker) ctorTakes(name string, x *syntax.Call) bool {
	cls := c.findClass(name)
	if cls == nil || cls.Object || cls.Interface {
		return false
	}
	cls.Ensure()
	return c.anyMayTake(userCtors(cls), x)
}

// invokeFitsType reports whether a value of type ft may be invoked with a
// call's arguments, as invokeFits. A value of another type needs an
// invoke operator that may take them: ktor's ByteReadChannel has a
// readBuffer property, a Source, and a readBuffer() extension.
func (c *Checker) invokeFitsType(ft types.Type, x *syntax.Call) bool {
	ft = ft.Flex()
	if !ft.IsFunc() {
		if x.Infix && ft.K != types.Invalid {
			// b f 3 calls a property f's invoke only if it is infix;
			// otherwise an infix extension f of b is called.
			return c.infixInvoke(ft, x)
		}
		return ft.K == types.Invalid || c.hasFun(ft, "invoke", x)
	}
	if x.Infix {
		// A function type's invoke isn't infix: b f 3 is an infix
		// extension f's call.
		return false
	}
	for _, n := range x.Names {
		if n != "" {
			return false
		}
	}
	n := ft.A.Len() - 1
	params := ft.A.List[:n]
	k := ft.Contexts()
	switch {
	case len(x.Args) == n:
	case k > 0 && len(x.Args) == n-k:
		params = params[k:] // implicit context arguments'
	case ft.IsExtFunc() && len(x.Args) == n-k-1 && c.implicitOf(params[k].T):
		params = params[k+1:] // and the implicit receiver's
	default:
		return false
	}
	for i, a := range x.Args {
		pt := params[i].T
		if _, ok := a.(*syntax.Lambda); ok && pt.K != types.TypeParam && !pt.Flex().IsFunc() {
			return false
		}
		if pt.HasParams() || pt.HasVars() {
			continue
		}
		if t, ok := c.peekType(a); ok && !c.subtype(t, pt) {
			return false
		}
	}
	return true
}

// infixInvoke reports whether a value of type ft, which isn't a function
// type, has an infix invoke operator that may take call x.
func (c *Checker) infixInvoke(ft types.Type, x *syntax.Call) bool {
	if cls := c.classOf(ft); cls != nil && !types.MayBeNull(ft) {
		cls.Ensure()
		for _, f := range c.accessible(cls, c.javaMethods(cls, "invoke")) {
			if f.Infix && c.mayTake(f, x) {
				return true
			}
		}
	}
	for _, cl := range c.extCalleesFor("invoke", ft, x) {
		if cl.f.Infix && c.mayTake(cl.f, x) {
			return true
		}
	}
	return false
}

// implicitOf reports whether an implicit receiver in scope, or this, may
// be of type t, which may mention type parameters.
func (c *Checker) implicitOf(t types.Type) bool {
	t = t.Flex()
	fits := func(r types.Type) bool {
		if t.HasParams() {
			if cls := c.classOf(t.NonNull()); cls != nil {
				_, ok := c.asSuper(r, cls)
				return ok
			}
			return true
		}
		return c.subtype(r, t)
	}
	for i := len(c.recvs) - 1; i >= 0; i-- {
		if fits(c.scope.narrowed(c.recvs[i].sym)) {
			return true
		}
	}
	return c.cls != nil && fits(c.cls.This())
}

// constructible reports whether a class has constructors to call.
func constructible(cls *types.ClassSym) bool {
	cls.Ensure()
	if cls.Abstract {
		return false
	}
	if !cls.External {
		return true
	}
	for _, f := range cls.Ctors {
		if !f.Protected {
			return true
		}
	}
	return false
}

// noSpread reports the spread operator in a call of a function without
// vararg parameters that resolve doesn't check.
func (c *Checker) noSpread(x *syntax.Call) {
	for _, a := range x.Args {
		if isSpread(a) {
			c.errorf(a.Pos(), "the spread operator (*foo) can only be applied in a vararg position")
		}
	}
}

// index checks x[i] as a call of the operator get, or with name set,
// x[i] = v as a call of set.
func (c *Checker) index(x *syntax.Index, name string, args []syntax.Expr, want types.Type) types.Type {
	// The index of a safe call is of its chain: a?.b[i] is a?.b?.get(i),
	// if b's type isn't nullable.
	safe := safeChain(x.X)
	m := &syntax.Member{X: x.X, Name: name, Safe: safe}
	m.P = x.P
	call := &syntax.Call{Fun: m, Args: args}
	call.P = x.P
	if name == "set" {
		call.PassLast() // the value
	}
	if c.indexCalls == nil {
		c.indexCalls = map[*syntax.Call]bool{}
	}
	c.indexCalls[call] = true
	x.Call = call
	t := c.expr(call, want)
	if f := call.Sym; f != nil && !f.Operator {
		c.errorf(x.P, "'operator' modifier is required on '%s' in '%s'", name, f.Owner.DisplayName())
	}
	if rt := chainType(x.X); safe && rt.K != types.Invalid && types.MayBeNull(rt) && !rt.P {
		c.errorf(x.P, "only safe (?.) or non-null asserted (!!.) calls are allowed on a nullable receiver of type %s", rt)
		return types.InvalidType
	}
	return t
}

// safeChain reports whether x ends a chain of safe calls that an index
// of it, x[i], continues: a?.b, a?.f(), or an index of one, a?.b[i].
// Parentheses end the chain: (a?.b)[i] indexes a nullable value.
func safeChain(x syntax.Expr) bool {
	switch x := x.(type) {
	case *syntax.Member:
		return x.Safe && !x.Paren
	case *syntax.Call:
		switch fn := x.Fun.(type) {
		case *syntax.Member:
			return !x.Invoke && fn.Safe && !x.Paren
		case *syntax.Call, *syntax.Index:
			return !x.Paren && safeChain(fn) // a?.f()(v), see safeChainInvoke
		}
		return false
	case *syntax.Index:
		return !x.Paren && safeChain(x.X)
	}
	return false
}

// chainType returns the type of x, the end of a chain of safe calls as
// safeChain reports, on a receiver that isn't null, or the invalid type
// if the checker didn't record it.
func chainType(x syntax.Expr) types.Type {
	switch x := x.(type) {
	case *syntax.Member:
		return x.SafeT
	case *syntax.Call:
		if m, ok := x.Fun.(*syntax.Member); ok {
			return m.SafeT
		}
	case *syntax.Index:
		if x.Call != nil {
			return chainType(x.Call)
		}
	}
	return types.InvalidType
}

// stdlibExt reports whether builtinMember's function name of receiver
// type rt is an extension of the standard library, not a member, which
// other extensions may come before.
func stdlibExt(rt types.Type, name string) bool {
	switch rt {
	case types.StringType:
		return name == "uppercase" || name == "lowercase" || name == "toInt" || name == "toCharArray"
	case types.CharType:
		_, ok := types.CharMethods[name]
		return ok
	}
	return false
}

// extShadows reports whether an extension name for receiver type rt that
// isn't the standard library's comes first for call x: a local one, or
// one of the program's or of its imports.
func (c *Checker) extShadows(name string, rt types.Type, x *syntax.Call) bool {
	for _, cl := range c.extCalleesFor(name, rt, x) {
		if !strings.HasPrefix(cl.f.Facade, "kotlin/") && c.mayTake(cl.f, x) {
			return true
		}
	}
	return false
}

// builtinShadowed reports whether an extension comes before the built-in
// operator function name of a number or a Char for call x, as extShadows
// reports: not when the member takes the argument, as members win over
// extensions. A unary one always does: this.inc() in an Int?.inc() for a
// this known not to be null, compareTo(c.code) in an Int.compareTo(Char).
func (c *Checker) builtinShadowed(name string, rt types.Type, x *syntax.Call) bool {
	if x.Infix && c.infixExtOnly(name, rt) { // see call.go
		return true
	}
	if len(x.Args) == 0 || !c.extShadows(name, rt, x) {
		return false
	}
	if t := c.argTypes(x)[0]; t.K != types.Invalid {
		if _, _, ok := types.OperatorFunType(name, rt, t.Flex()); ok {
			return false
		}
	}
	return true
}

func (c *Checker) builtinMember(x *syntax.Call, fn *syntax.Member, rt types.Type, want types.Type, sink *system) types.Type {
	if rangeMember(rt, fn.Name, len(x.Args)) && !fn.Safe && !(x.Infix && c.infixExtOnly(fn.Name, rt)) {
		infix := x.Infix
		if t, ok := c.memberRange(x, fn, rt, want); ok {
			if infix {
				c.errorf(fn.P, "'infix' modifier is required on '%s'", fn.Name)
			}
			return t
		}
	}
	if n, ok := types.OperatorFuns[fn.Name]; ok && (rt.IsNumeric() || rt == types.CharType) && len(x.Args) == n && !c.builtinShadowed(fn.Name, rt, x) {
		// An operator function called by name, 1.plus(2), is the
		// operator: 1 + 2.
		c.noSpread(x)
		at := types.InvalidType
		if n == 1 {
			at = c.value(x.Args[0], noWant).Flex()
		}
		if _, res, ok := types.OperatorFunType(fn.Name, rt, at); ok {
			x.Builtin = types.OperatorFun
			c.builtinInfix(x, fn)
			return res
		}
		if at.K == types.Invalid {
			return types.InvalidType
		}
		if cands := c.extCallees(fn.Name, rt); len(cands) > 0 {
			x.Args[0] = checked(x.Args[0], at)
			return c.resolve(x, cands, want, sink)
		}
		c.errorf(x.Args[0].Pos(), "type mismatch: inferred type is %s but %s was expected", at, rt)
		return types.InvalidType
	}
	type sig struct {
		b      types.Builtin
		params []types.Type
		ret    types.Type
	}
	var s *sig
	switch {
	case rt == types.StringType:
		switch fn.Name {
		case "uppercase":
			s = &sig{types.StrUppercase, nil, types.StringType}
		case "lowercase":
			s = &sig{types.StrLowercase, nil, types.StringType}
		case "isEmpty":
			s = &sig{types.StrIsEmpty, nil, types.BoolType}
		case "substring":
			if len(x.Args) == 1 {
				s = &sig{types.StrSubstring, []types.Type{types.IntType}, types.StringType}
			} else {
				s = &sig{types.StrSubstring, []types.Type{types.IntType, types.IntType}, types.StringType}
			}
		case "toInt":
			s = &sig{types.ToInt, nil, types.IntType}
		case "toString":
			s = &sig{types.ToString, nil, types.StringType}
		case "get":
			s = &sig{types.StrGet, []types.Type{types.IntType}, types.CharType}
		case "plus":
			s = &sig{types.StrPlus, []types.Type{types.AnyType.Nullable()}, types.StringType}
		case "toCharArray":
			s = &sig{types.StrToCharArray, nil, types.ArrayOf(types.CharType)}
		}
	case (rt == types.IntType || rt == types.LongType) && types.BitwiseOps[fn.Name]:
		switch fn.Name {
		case "inv":
			s = &sig{types.Bitwise, nil, rt}
		case "shl", "shr", "ushr":
			s = &sig{types.Bitwise, []types.Type{types.IntType}, rt}
		default:
			s = &sig{types.Bitwise, []types.Type{rt}, rt}
		}
	case rt.IsNumeric() || rt == types.CharType:
		if b, t, ok := conversion(fn.Name); ok {
			s = &sig{b, nil, t}
			if via := deprecatedConversion(rt, b); via != "" {
				c.errorf(fn.P, "'%s' is deprecated: convert to %s first", fn.Name, via)
			}
		} else if m, ok := types.CharMethods[fn.Name]; ok && rt == types.CharType {
			ret := types.CharType
			if strings.HasPrefix(m, "is") {
				ret = types.BoolType
			}
			s = &sig{types.CharMethod, nil, ret}
		}
	case rt == types.BoolType && fn.Name == "toString":
		s = &sig{types.ToString, nil, types.StringType}
	case rt == types.BoolType && (fn.Name == "and" || fn.Name == "or" || fn.Name == "xor"):
		// Boolean's eager logical functions: iand, ior and ixor.
		s = &sig{types.Bitwise, []types.Type{types.BoolType}, types.BoolType}
	case rt == types.BoolType && fn.Name == "not":
		s = &sig{types.Bitwise, nil, types.BoolType} // !x, as a call
	}
	if s != nil && stdlibExt(rt, fn.Name) && c.extShadows(fn.Name, rt, x) {
		s = nil // as ktor's Char.isLowerCase, of a star import
	}
	if s != nil && x.Infix && s.b != types.Bitwise && c.infixExtOnly(fn.Name, rt) {
		s = nil // an infix extension, which the member that isn't infix leaves the call to
	}
	if s == nil {
		object := c.u.ext.class("java/lang/Object")
		arity := 0
		if fn.Name == "equals" {
			arity = 1
		}
		if object != nil && anyMember[fn.Name] && x.Names == nil && len(x.Args) == arity && !x.Infix {
			// Any's member, which wins over extensions: s.equals(1) is
			// not String?.equals(String?).
			return c.resolve(x, c.memberCallees(c.javaMethods(object, fn.Name), types.AnyType), want, sink)
		}
		if str := c.u.ext.class("java/lang/String"); str != nil && rt == types.StringType && fn.Name == "subSequence" && x.Names == nil && len(x.Args) == 2 {
			// String's member subSequence(startIndex, endIndex), which
			// wins over the standard library's deprecated extension
			// String.subSequence(start, end).
			return c.resolve(x, c.memberCallees(c.javaMethods(str, fn.Name), types.StringType), want, sink)
		}
		if str := c.u.ext.class("java/lang/String"); str != nil && rt == types.StringType && fn.Name == "compareTo" && x.Names == nil && len(x.Args) == 1 && !x.Infix && !hasLambda(x.Args) {
			// String's member compareTo(other: String), which wins over
			// extensions when it takes the argument, also a call's:
			// a.compareTo(name()) isn't an operator String.compareTo(Version).
			at := c.value(x.Args[0], noWant)
			x.Args[0] = checked(x.Args[0], at)
			if at.K != types.Invalid && c.subtype(at, types.StringType) {
				return c.resolve(x, c.memberCallees(c.javaMethods(str, fn.Name), types.StringType), want, sink)
			}
		}
		if cands := c.extCalleesFor(fn.Name, rt, x); len(cands) > 0 {
			return c.resolve(x, cands, want, sink)
		}
		if object != nil && anyMember[fn.Name] {
			// Any's members, of a number or a function value: boxed,
			// so that 0.0.equals(-0.0) is false, as in Kotlin.
			return c.resolve(x, c.memberCallees(c.javaMethods(object, fn.Name), types.AnyType), want, sink)
		}
		if t, ok := c.inaccessibleOuterCall(fn.P, fn.Name, &rt); ok {
			c.args(x, nil)
			return t
		}
		c.errorf(fn.P, "unresolved reference: %s", fn.Name)
		c.args(x, nil)
		return types.InvalidType
	}
	if len(x.Args) != len(s.params) {
		// An extension with other parameters: Byte.toString(radix).
		if cands := c.extCalleesFor(fn.Name, rt, x); len(cands) > 0 {
			return c.resolve(x, cands, want, sink)
		}
	}
	c.noSpread(x)
	if len(x.Args) != len(s.params) {
		c.errorf(x.P, "wrong number of arguments for %s", fn.Name)
		c.args(x, nil)
		return s.ret
	}
	for i, a := range x.Args {
		at := c.value(a, s.params[i])
		if at.K != types.Invalid && !c.subtype(at, s.params[i]) {
			// An extension that takes it: okio's infix Int.and(Long).
			if cands := c.extCallees(fn.Name, rt); len(cands) > 0 {
				x.Args[i] = checked(a, at)
				return c.resolve(x, cands, want, sink)
			}
		}
		c.assignable(at, s.params[i], a.Pos())
	}
	x.Builtin = s.b
	c.builtinInfix(x, fn)
	return s.ret
}

// builtinInfix reports an infix call x of a built-in member that isn't
// infix: 1 plus 1, "a" get 0. Only the bitwise functions of Int, Long
// and Boolean (and, or, xor, shl, shr, ushr) are infix.
func (c *Checker) builtinInfix(x *syntax.Call, fn *syntax.Member) {
	if x.Infix && x.Builtin != types.Bitwise {
		c.errorf(fn.P, "'infix' modifier is required on '%s'", fn.Name)
	}
}

// conversion returns the builtin of a number's or Char's conversion
// function, such as toInt, and its result type.
func conversion(name string) (types.Builtin, types.Type, bool) {
	switch name {
	case "toInt":
		return types.ToInt, types.IntType, true
	case "toLong":
		return types.ToLong, types.LongType, true
	case "toDouble":
		return types.ToDouble, types.DoubleType, true
	case "toFloat":
		return types.ToFloat, types.FloatType, true
	case "toByte":
		return types.ToByte, types.ByteType, true
	case "toShort":
		return types.ToShort, types.ShortType, true
	case "toChar":
		return types.ToChar, types.CharType, true
	case "toString":
		return types.ToString, types.StringType, true
	}
	return types.NotBuiltin, types.InvalidType, false
}

// deprecatedConversion returns the type that a conversion Kotlin forbids
// should go through: Int for Long.toChar() and Double.toByte().
func deprecatedConversion(from types.Type, b types.Builtin) string {
	switch {
	case b == types.ToChar && from != types.IntType && from != types.CharType:
		return "Int"
	case (b == types.ToByte || b == types.ToShort) && (from == types.FloatType || from == types.DoubleType):
		return "Int"
	}
	return ""
}

// bareEmpty reports whether b is an empty branch without braces.
func bareEmpty(b *syntax.Block) bool {
	return len(b.Stmts) == 0 && b.End == (syntax.Pos{})
}

func (c *Checker) ifExpr(x *syntax.If, want types.Type, valueUsed bool) types.Type {
	sink := c.takeBranchSink()
	defer c.sinkLambdas(sink != nil && c.branchArg)()
	c.value(x.Cond, types.BoolType)
	c.wantBool(x.Cond)
	whenTrue, whenFalse := c.narrowing(x.Cond)
	if valueUsed && !c.tailIfs[x] && x.Else != nil && (bareEmpty(x.Then) || bareEmpty(x.Else)) {
		// if (c) else x, or if (c) x else;, as kotlinc's
		// INVALID_IF_AS_EXPRESSION: an empty branch without braces is no
		// value, where {} is Unit.
		c.errorf(x.P, "'if' must have both main and 'else' branches if used as an expression")
	}

	// The vars the branches assign have the join of their types at the
	// ends of the branches after the if. So have the values the condition
	// narrows, when there is an else: ktor's prev, a Node after (if (prev
	// === this) findHead() else (prev as Node)).
	vars := c.watchedVars(x.Then, x.Else)
	if x.Else != nil {
		for _, n := range append(whenTrue[:len(whenTrue):len(whenTrue)], whenFalse...) {
			if n.recv == nil && n.sym != nil && narrowable(n.sym.Type) && !slices.Contains(vars, n.sym) {
				vars = append(vars, n.sym)
			}
		}
	}
	var exits []*branchExit
	defer func() { c.merge(vars, exits) }()
	var pending []pendingBranch
	c.push()
	c.scope.narrows = whenTrue
	exits = append(exits, c.watchExit(vars))
	thenT := c.ifBranch(x.Then, want, valueUsed, sink == nil, &pending)
	c.pop()
	if x.Else == nil {
		exits = append(exits, c.startExit(vars, whenFalse))
	}
	if x.Else == nil {
		if valueUsed && !c.tailIfs[x] {
			c.errorf(x.P, "'if' must have both main and 'else' branches if used as an expression")
			return types.InvalidType
		}
		c.completeBranches(pending, types.NothingType, noWant, false, false)
		return types.UnitType
	}
	c.push()
	c.scope.narrows = whenFalse
	exits = append(exits, c.watchExit(vars))
	elseT := c.ifBranch(x.Else, lambdaBranchWant(x.Else, thenT, want, valueUsed), valueUsed, sink == nil, &pending)
	c.pop()
	jumps := jumped(pending, []types.Type{thenT, elseT}, []*syntax.Block{x.Then, x.Else})
	if !valueUsed {
		c.completeBranches(pending, c.unusedJoin(types.NothingType, x.Then, x.Else), noWant, false, jumps)
		c.noteBranchJoin(x, c.statementJoin(x.Then, x.Else))
		thenT, elseT = inferredJump(thenT, x.Then), inferredJump(elseT, x.Else)
		if thenT.K == types.Nothing && elseT.K == types.Nothing {
			return types.NothingType
		}
		return types.UnitType
	}
	t, ok := c.joinBranches(thenT, elseT, want, x.P)
	if !ok {
		c.errorf(x.P, "type mismatch: 'if' branches have incompatible types %s and %s", thenT, elseT)
		return types.InvalidType
	}
	if sink != nil {
		return c.sinkBranches(sink, pending, t)
	}
	t = c.completeBranches(pending, t, want, true, jumps)
	if want.K == types.Invalid {
		c.noteBranchJoin(x, t)
	}
	return t
}

// joinBranches joins the type of a branch of an if or when, bt, with the
// result of the branches before it, as joinBranch. A Unit and a null give
// a Unit?, kotlin.Unit's class: when { b -> foo() else -> null }.
func (c *Checker) joinBranches(result, bt, want types.Type, p syntax.Pos) (types.Type, bool) {
	if u := c.unitOrNull(result, bt); u.K != types.Invalid {
		return u, true
	}
	return c.joinBranch(result, bt, want, p)
}

// unitOrNull returns Unit?, kotlin.Unit's class, if one of a and b is a
// Unit and the other null, or Invalid.
func (c *Checker) unitOrNull(a, b types.Type) types.Type {
	if a.K == types.Null {
		a, b = b, a
	}
	if a.K != types.Unit || a.N || b.K != types.Null {
		return types.InvalidType
	}
	if u := c.u.ext.class("kotlin/Unit"); u != nil {
		return types.ClassType(u).Nullable()
	}
	return types.InvalidType
}

// blockLambda returns the lambda that a branch's block is, or nil.
func blockLambda(b *syntax.Block) *syntax.Lambda {
	if b == nil || len(b.Stmts) != 1 {
		return nil
	}
	if s, ok := b.Stmts[0].(*syntax.ExprStmt); ok {
		l, _ := s.X.(*syntax.Lambda)
		return l
	}
	return nil
}

// pendingBranch is a branch of an if or when whose value is a generic
// call that can't be inferred on its own, as listOf() in
// if (c) listOf(x) else listOf().
type pendingBranch struct {
	sys  *system
	t    types.Type
	body *syntax.Block
	lit  syntax.Expr // an integer literal's value, as 0 in if (c) 1L else 0
	// lambda is set for a branch that is only a lambda without its
	// parameters' types, which, as K2's postponed argument, takes them
	// from the others' function type: when { a -> {{ x -> }} b -> f }.
	lambda bool
	// sunk is set for a lambda branch postponed with the call the if or
	// when is an argument of.
	sunk bool
	// open is set when the call needs the other branches' types: it has
	// no lambdas, or they don't give their parameter types. See openLast.
	open bool
	// unused is set when the branch's value isn't used: its block's type
	// stays the statement's.
	unused bool
	// scope is the branch's block's, with its locals and smart casts,
	// which its lambdas see.
	scope *scope
	// free is the builder's run the call's variables are related to, if
	// nothing constrained that run's variables yet when the branch was
	// checked (see freeStubRun).
	free   *stubRun
	freeAt int // the number of free's constraints then
}

// branch checks a branch of an if or when. Without a known expected
// type, or with Any, a branch that is a single generic call is left
// incomplete if it can't be inferred on its own: Kotlin infers the
// branches together, so the others' types complete it, as Success's in
// if (c) closed(cause) else success(x) for an Any?, and build {}'s in
// if (c) build { yield(x) } else build {}, also unused. It is then seen as
// Nothing, which the join ignores, until completeBranches.
func (c *Checker) branch(b *syntax.Block, want types.Type, valueUsed bool, pending *[]pendingBranch) types.Type {
	return c.ifBranch(b, want, valueUsed, false, pending)
}

// ifBranch checks a branch of an if, a when or a try, as branch. With
// together, which the branches' own inference (not a call's they are an
// argument of) sets, a used branch's call whose variables only their
// upper bounds would fix is left incomplete too: K2's synthetic call
// fixes the result's variable first, from the other branches' types, and
// then the call's below it, as T of <T : Marker> get(key) in
// if (c) get(key, T::class.java) else get(key) is the other branch's T,
// not Marker.
func (c *Checker) ifBranch(b *syntax.Block, want types.Type, valueUsed, together bool, pending *[]pendingBranch) types.Type {
	if c.lambdaSink && valueUsed && want.K == types.Invalid && len(b.Stmts) == 1 {
		// A lambda of an if or a when that is a call's argument is, as
		// K2's synthetic call's, postponed with the call's: { -> } of
		// Foo(if (b) { -> } else null) for an expected Foo<suspend ()
		// -> Unit> is a suspend lambda (see sinkBranches).
		if es, ok := b.Stmts[0].(*syntax.ExprStmt); ok {
			if l, ok := es.X.(*syntax.Lambda); ok && l.Arrow && len(l.Params) == 0 && !l.Anon && l.Ref == nil && !l.SuspendMod {
				*pending = append(*pending, pendingBranch{body: b, lambda: true, sunk: true, scope: c.scope})
				return types.NothingType
			}
		}
	}
	if n := len(b.Stmts); n > 0 && want.Flex().E {
		if es, ok := b.Stmts[n-1].(*syntax.ExprStmt); ok {
			if l, ok := es.X.(*syntax.Lambda); ok && l.Arrow && !l.Anon {
				c.markRecvParam(l)
			}
		}
	}
	if want.K != types.Invalid && want.K != types.Any && !want.HasVars() || len(b.Stmts) == 0 {
		return c.block(b, want, valueUsed)
	}
	es, ok := b.Stmts[len(b.Stmts)-1].(*syntax.ExprStmt)
	if !ok {
		return c.block(b, want, valueUsed)
	}
	if l, ok := es.X.(*syntax.Lambda); ok && valueUsed && len(b.Stmts) == 1 && l.Arrow && len(l.Params) > 0 && untypedParams(l) && want.K == types.Invalid {
		*pending = append(*pending, pendingBranch{body: b, lambda: true, scope: c.scope})
		return types.NothingType
	}
	if valueUsed && literalArith(es.X) {
		// An integer literal takes the type of the other branches if it
		// can: 0 is a Long in if (c) 1L else 0.
		t := c.block(b, want, valueUsed)
		*pending = append(*pending, pendingBranch{t: t, body: b, lit: es.X})
		return types.NothingType
	}
	if _, call := es.X.(*syntax.Call); !call {
		return c.block(b, want, valueUsed)
	}
	// The call is the branch's last statement, which the system infers:
	// the statements before it are checked as usual.
	inner := newSystem(c)
	if len(b.Stmts) == 1 {
		c.sink = inner
	} else {
		c.tailBlock, c.tailSink = b, inner
	}
	bt := c.block(b, want, valueUsed)
	c.sink, c.tailBlock, c.tailSink = nil, nil, nil
	t := bt
	if !valueUsed {
		t = es.X.Type() // the call's, which the others' constrain
	}
	if len(inner.vars) == 0 && len(inner.fixups) == 0 {
		return bt
	}
	if len(inner.lambdas) == 0 && inner.selfInferable(t) && !(together && valueUsed && !want.HasVars() && inner.upperOnly(t)) {
		inner.finish()
		if !valueUsed {
			return bt
		}
		t = inner.final(t)
		b.T = t
		return t
	}
	free, freeAt := c.freeStubRun(inner), 0
	if free != nil {
		freeAt = len(free.cons)
	}
	*pending = append(*pending, pendingBranch{sys: inner, t: t, body: b, scope: c.blockScope, unused: !valueUsed, free: free, freeAt: freeAt})
	if !valueUsed {
		return bt
	}
	return types.NothingType
}

// branchLambda checks pending branch p, a lambda (see pendingBranch),
// with the others' type, result, if it is a function type, or want.
func (c *Checker) branchLambda(p pendingBranch, result, want types.Type) types.Type {
	target := result
	if !target.Flex().IsFunc() || target.HasVars() {
		target = want
	}
	saved := c.scope
	c.scope = p.scope
	defer func() { c.scope = saved }()
	return c.block(p.body, target, true)
}

// completeBranches completes the pending branches with the type of the
// others, result, or the expected type, and returns the type of all. If
// the value isn't used, only the open ones take the others' type.
//
// jumps is set when one of the other branches is Nothing, as a jump: as
// K2's synthetic call's K, which a Nothing or a null fixes when nothing
// else does, a call's variables are then Nothing, or Nothing? with a
// null: T of make() in if (c) make() else TODO().
func (c *Checker) completeBranches(pending []pendingBranch, result, want types.Type, valueUsed, jumps bool) types.Type {
	c.openLast(pending)
	done := types.NothingType // the completed pending branches' type
	for _, p := range pending {
		if p.lambda {
			bt := c.branchLambda(p, result, want)
			if j, ok := c.join(result, bt); ok {
				result = j
			}
			continue
		}
		if p.lit != nil {
			bt := p.t
			switch r := result.Flex().NonNull(); r {
			case types.LongType, types.ShortType, types.ByteType:
				bt = c.expr(p.lit, r)
			}
			p.body.T = bt
			if j, ok := c.join(result, bt); ok {
				result = j
			}
			continue
		}
		target := result
		if p.unused && len(p.sys.lambdas) > 0 {
			// A builder's takes only the other builders' types: see
			// unusedJoin.
			target = done
		}
		if target.K == types.Nothing || target.K == types.Null || target.K == types.Invalid || target.HasVars() {
			// Nothing, or null's Nothing?, which the branches join with.
			target = want
		}
		jumpTarget := false
		if target.K == types.Invalid && (jumps && result.K == types.Nothing || result.K == types.Null) && len(p.sys.lambdas) == 0 {
			target, jumpTarget = result, true
		}
		forced := false
		if target.K != types.Invalid && !target.HasVars() && (valueUsed || p.open) {
			if trial := c.branchTrial(p.sys, p.t, target); trial != nil { // see expected.go
				p.sys = trial
			} else if !jumpTarget {
				// Not the Unit that a lambda's unused last statement is
				// coerced to: put("a", listOf(1)) at the end of buildMap's
				// lambda doesn't make V? a Unit.
				forced = p.free != nil && !(p.unused && target.K == types.Unit)
			}
		}
		// The branch's lambdas, as sequence { yield(1) }'s in
		// if (c) sequence { yield(1) } else seq, in its scope.
		saved := c.scope
		if p.scope != nil {
			c.scope = p.scope
		}
		c.runLambdas(p.sys, false)
		c.scope = saved
		p.sys.finish()
		if forced && slices.Contains(p.free.joined, p.sys) {
			// K2 infers the call with the builder's variables, which
			// nothing bounded when the branch was checked: the join of
			// the branches is its upper bound whatever the lambda does
			// after, as add(42) after removeLast() in an if's branches
			// is a mismatch for its Boolean.
			p.free.force(p.freeAt, p.t, target)
		}
		bt := p.sys.final(p.t)
		if !p.unused {
			p.body.T = bt
		}
		if j, ok := c.join(result, bt); ok {
			result = j
		}
		if j, ok := c.join(done, bt); ok {
			done = j
		}
	}
	return result
}

// jumped reports whether one of bodies, the branches of an if or when of
// types ts, isn't pending and is Nothing: it jumps.
func jumped(pending []pendingBranch, ts []types.Type, bodies []*syntax.Block) bool {
	for i, b := range bodies {
		if b != nil && ts[i].K == types.Nothing && !slices.ContainsFunc(pending, func(p pendingBranch) bool { return p.body == b }) {
			return true
		}
	}
	return false
}

// incompatibleCond renames errs, the errors of a when's condition
// compared with its subject, as kotlinc's INCOMPATIBLE_TYPES, where an ==
// is EQUALITY_NOT_APPLICABLE: when (a) { true -> } for an a of an
// interface type A has incompatible types A and Boolean.
func incompatibleCond(errs []*syntax.Error) {
	const eq = "operator '==' cannot be applied to "
	for _, e := range errs {
		if rest, ok := strings.CutPrefix(e.Msg, eq); ok {
			e.Msg = "incompatible types " + rest
		}
	}
}

// noteBranchJoin records t, the join of the types of the branches of
// the if or when x, which no expected type gave a type, for the warnings
// about it (see inferredInvisibleBranches).
func (c *Checker) noteBranchJoin(x syntax.Expr, t types.Type) {
	if t.K == types.Invalid {
		return
	}
	if c.branchJoins == nil {
		c.branchJoins = map[syntax.Expr]types.Type{}
	}
	c.branchJoins[x] = t
}

// statementJoin returns the join of the values of bodies, the branches of
// an if or when whose value isn't used, or InvalidType unless each that
// doesn't jump ends with an expression of a type that isn't Unit.
func (c *Checker) statementJoin(bodies ...*syntax.Block) types.Type {
	result := types.NothingType
	for _, b := range bodies {
		if b == nil || len(b.Stmts) == 0 {
			return types.InvalidType
		}
		if b.T.K == types.Nothing {
			continue
		}
		es, ok := b.Stmts[len(b.Stmts)-1].(*syntax.ExprStmt)
		if !ok {
			return types.InvalidType
		}
		bt := es.X.Type()
		switch {
		case bt.K == types.Invalid || bt.K == types.Unit || bt.HasVars():
			return types.InvalidType
		case bt.K == types.Nothing:
		case result.K == types.Nothing:
			result = bt
		default:
			j, ok := c.join(result, bt)
			if !ok {
				return types.InvalidType
			}
			result = j
		}
	}
	return result
}

// unusedJoin joins the type of a branch of an if or when whose value
// isn't used into the others', result, which complete a generic call
// that can't be inferred on its own, as in K2, which infers the
// branches together: T of fun <T : S> isEqualTo(): T is the Path in
// if (c) path else e.isEqualTo(). Pending branches, of variables' types,
// and Unit ones don't count, and a builder's lambdas, not these types,
// give its variables, as without them.
func (c *Checker) unusedJoin(result types.Type, bodies ...*syntax.Block) types.Type {
	for _, b := range bodies {
		if b == nil || len(b.Stmts) == 0 || b.T.K == types.Nothing {
			continue
		}
		es, ok := b.Stmts[len(b.Stmts)-1].(*syntax.ExprStmt)
		if !ok || es.X.Type().K == types.Invalid {
			continue
		}
		bt := es.X.Type()
		if bt.K == types.Invalid || bt.K == types.Unit || bt.K == types.Nothing || bt.HasVars() {
			continue
		}
		if result.K == types.Nothing {
			result = bt
		} else if j, ok := c.join(result, bt); ok {
			result = j
		}
	}
	return result
}

// openLast orders pending branches, when more than one is, so that the
// open ones come last and take the others' types. As in Kotlin, builder
// lambdas give their variables first: their constraint pass (see
// stubInference) runs before.
func (c *Checker) openLast(pending []pendingBranch) {
	if len(pending) < 2 {
		for i := range pending {
			pending[i].open = true
		}
		return
	}
	saved := c.scope
	for i := range pending {
		p := &pending[i]
		if p.sys == nil {
			continue
		}
		p.open = len(p.sys.lambdas) == 0
		for _, pl := range p.sys.lambdas {
			ft := p.sys.apply(pl.p)
			if !ft.Flex().IsFunc() {
				if m, f := c.samTarget(ft); m != nil {
					ft = f
				}
			}
			params, _, ok := funcParts(ft)
			if !ok {
				continue
			}
			c.scope = pl.scope
			c.stubInference(p.sys, pl.l, params, ft, false)
			p.open = p.open || !slices.ContainsFunc(params, p.sys.known)
		}
	}
	c.scope = saved
	slices.SortStableFunc(pending, func(a, b pendingBranch) int {
		switch {
		case a.open == b.open:
			return 0
		case a.open:
			return 1
		}
		return -1
	})
}

func (c *Checker) whenExpr(x *syntax.When, want types.Type, valueUsed bool) types.Type {
	sink := c.takeBranchSink()
	defer c.sinkLambdas(sink != nil && c.branchArg)()
	var pending []pendingBranch
	softFail := false
	// As after an if, the vars the branches assign have the join of their
	// types: merged in the enclosing scope, after the pop below.
	vars := c.watchedVars(x)
	var exits []*branchExit
	defer func() { c.merge(vars, exits) }()
	var st types.Type
	// subj is the stable value that conditions on the subject smart cast:
	// the subject's variable, a variable or a stable property.
	var subj narrow
	hasSubj := false
	subjType := func() types.Type { return c.scope.narrowedMember(subj.recv, subj.sym, subj.t) }
	// afterNull is what the code after the when learns if its null
	// branch jumps: the subject isn't null, as t after
	// when (t?.tier) { A -> 1; null -> return }.
	var afterNull []narrow
	defer func() { c.scope.narrows = append(c.scope.narrows, afterNull...) }()
	c.push() // the subject variable's scope
	defer c.pop()
	if x.Subject != nil {
		if x.SubjVar != "" {
			if x.SubjType != nil {
				declared := c.resolveType(x.SubjType)
				c.assignable(c.value(x.Subject, declared), declared, x.Subject.Pos())
				st = declared
			} else {
				st = c.value(x.Subject, noWant)
			}
			x.SubjSym = &types.VarSym{Name: x.SubjVar, Type: st, Kind: types.Local}
			c.scope.declare(x.SubjSym)
			c.bindValue(x.SubjSym, x.Subject, x.SubjType != nil)
			subj, hasSubj = narrow{sym: x.SubjSym, t: st}, true
		} else {
			st = c.value(x.Subject, noWant)
			x.SubjSym = &types.VarSym{Name: "<subject>", Type: st, Kind: types.Local}
			subj, hasSubj = c.narrowKeyOf(x.Subject)
			if !hasSubj {
				// Not a stable value, but the subject's own value, which the
				// conditions test, is: not null after null -> in
				// when (getDateOrNull()) { null -> 0; in a..b -> 1 }.
				subj, hasSubj = narrow{sym: x.SubjSym, t: st}, true
			}
		}
	}
	hasElse := false
	var cov coverage
	result := types.NothingType
	allJump := true
	unused := types.NothingType     // the join of the branches, whose value isn't used: see unusedJoin
	var later []narrow              // what the failed conditions of the branches so far imply
	nullFirst := true               // no branch so far matches null
	funcBranch := types.InvalidType // see lambdaBranchWant
	var bts []types.Type            // the branches' types
	anyJump := false                // a branch that isn't pending jumps
	for i, br := range x.Branches {
		if br.Else && br.Guard == nil { // else if guard -> can be anywhere
			if i != len(x.Branches)-1 {
				c.errorf(br.P, "'else' must be the last branch")
			}
			hasElse = true
		}
		c.push()
		c.scope.narrows = append([]narrow(nil), later...)
		var whenTrue []narrow
		single := len(br.Conds) == 1
		// A guarded branch can fail when its conditions hold: they cover
		// nothing, and later branches learn nothing from them.
		saved := cov
		if br.Guard != nil {
			cov = coverage{}
		}
		laterLen := len(later)
		nonNull := 0
		for j := range br.Conds {
			cond := &br.Conds[j]
			switch {
			case cond.Is || cond.NotIs:
				if x.Subject == nil {
					c.errorf(cond.P, "'is' conditions need a 'when' subject")
					continue
				}
				cond.Target = c.checkTarget(cond.Type, st, nil, true)
				c.whenIsCheck(x, cond, st)
				if cond.Is && cond.Target.K == types.Class {
					cov.classes = append(cov.classes, cond.Target.C)
				}
				switch {
				case cond.Is && cond.Target.K != types.Invalid:
					cov.types = append(cov.types, cond.Target)
				case cond.NotIs && cond.Target.K == types.Class && !cond.Target.N:
					cov.not = append(cov.not, cond.Target.C)
				}
				if hasSubj && cond.Is {
					// What the subject is in the branches after one it
					// isn't: null after is Any.
					if t, ok := notIsNarrowed(subjType(), st, cond.Target); ok {
						n := narrow{recv: subj.recv, sym: subj.sym, t: t}
						later = append(later, n)
						if t.K != types.Null && t.K != types.Nothing {
							later = append(later, c.impliedBy(n)...)
						}
					}
				}
				if hasSubj && single && cond.Target.K != types.Invalid {
					n := narrow{recv: subj.recv, sym: subj.sym, t: c.isNarrowed(subjType(), cond.Target)}
					if cond.Is {
						// And what it implies, as an if does: x.data of
						// val y = x.data, when (y) { is String -> }.
						whenTrue = append(append(whenTrue, n), c.impliedBy(n)...)
					} else {
						later = append(append(later, n), c.impliedBy(n)...)
					}
				}
			case cond.In || cond.NotIn:
				if x.Subject == nil {
					c.errorf(cond.X.Pos(), "'in' conditions need a 'when' subject")
					c.value(cond.X, noWant)
					break
				}
				// The subject as the branches before smart cast it: not
				// null after null ->, in when (d) { null -> 0; in r -> 1 }.
				st := st
				if nt := subjType(); hasSubj && nt.K != types.Invalid {
					st = nt
				}
				r, ok := cond.X.(*syntax.Binary)
				var rt types.Type
				if ok && isRangeOp(r.Op) && r.Op != syntax.STEP {
					if rt = c.rangeExpr(cond.X); rt.K != types.Range && rt.K != types.Invalid {
						cond.X = checked(cond.X, rt) // a range of another type
						ok = false
					} else if c.nullableIn(st, rt) {
						cond.X, ok = c.rangeAsValue(r), false
					}
				}
				if !ok || !isRangeOp(r.Op) {
					// X.contains(subject)
					id := &syntax.Ident{Name: x.SubjSym.Name, Sym: x.SubjSym}
					id.P, id.T = cond.X.Pos(), st
					m := &syntax.Member{X: cond.X, Name: "contains"}
					m.P = cond.X.Pos()
					call := &syntax.Call{Fun: m, Args: []syntax.Expr{checked(id, st)}, Operator: true}
					call.P = cond.X.Pos()
					cond.Call = call
					if t := c.expr(call, types.BoolType); t.K != types.Invalid && t.Flex() != types.BoolType {
						c.errorf(cond.X.Pos(), "'contains' must return Boolean, not %s", t)
					}
					break
				} else if r.Op == syntax.STEP {
					c.errorf(cond.X.Pos(), "'in' is only supported with ranges written in place, without 'step'")
					rt = c.rangeExpr(cond.X)
				}
				if e := rt.RangeElem(); st.IsIntersection() && slices.Contains(st.Param().Bounds, e) {
					// A T smart cast to Char, a Char & T, as a Char.
					st = e
				}
				if w := rt.RangeElem(); !(isIntOrLong(st) && isIntOrLong(w)) {
					// At the condition, in 1..10, which takes the subject
					// as its argument.
					if t := st.Flex(); t != w && t.K != types.Invalid && w.K != types.Invalid {
						c.errorf(cond.P, "type mismatch: inferred type is %s but %s was expected", t, w)
					}
				}
			case x.Subject != nil:
				ct := c.value(cond.X, st)
				cond.OpType = types.InvalidType
				if nt := subjType(); hasSubj && floatOf(st).K == types.Invalid && floatOf(ct).K != types.Invalid && (nt.K == types.Double || nt.K == types.Float) {
					// The subject smart cast to a number by the branches
					// before: when (y) { !is Double -> ...; 0.0 -> ... }
					// compares Doubles as IEEE 754 says, a Float and a
					// Double as Doubles.
					cond.SubjType = nt
					if nt != floatOf(ct) && !types.MayBeNull(ct) {
						cond.OpType = types.DoubleType
					} else if nt == floatOf(ct) {
						cond.OpType = c.equality(cond.X.Pos(), syntax.EQ, nt, ct)
					} else {
						cond.SubjType = types.Type{}
					}
				}
				if cond.OpType.K == types.Invalid {
					n := len(c.errs)
					cond.OpType = c.equality(cond.X.Pos(), syntax.EQ, st, ct)
					incompatibleCond(c.errs[n:])
					if dt, ok := declaredOf(x.Subject); ok && cond.OpType.K == types.Invalid {
						// A smart-cast subject, compared with a value its
						// declared type may be equal to: as kotlinc, only
						// a warning (see whenEnumWarnings).
						m := len(c.errs)
						if t := c.equality(cond.X.Pos(), syntax.EQ, dt, ct); t.K != types.Invalid {
							c.errs, cond.OpType = c.errs[:n], t
						} else {
							c.errs = c.errs[:m]
						}
					}
				}
				cov.add(cond.X)
				if single && x.SubjVar == "" {
					// when (true) { c -> }, when (c) { true -> }: c's.
					eq := &syntax.Binary{Op: syntax.EQ, X: x.Subject, Y: cond.X}
					if t, f, ok := c.boolEquality(eq); ok {
						whenTrue, later = append(whenTrue, t...), append(later, f...)
					}
				}
				if equalsNonNull(ct) {
					nonNull++
				}
				if nonNull == len(br.Conds) {
					// Equal to a value that isn't null, or to one of
					// several: neither the subject nor the receivers of
					// its safe calls are.
					if hasSubj && types.MayBeNull(subjType()) {
						n := narrow{recv: subj.recv, sym: subj.sym, t: subjType().NonNull()}
						whenTrue = append(append(whenTrue, n), c.impliedBy(n)...)
					}
					whenTrue = append(whenTrue, c.safeReceivers(x.Subject)...)
				}
				if _, ok := cond.X.(*syntax.NullLit); ok {
					if hasSubj && st.K != types.Null && !definitelyNonNull(st) {
						// The branches before left it null alone, or
						// ruled null out.
						if nt := subjType(); nt.K == types.Null || definitelyNonNull(nt) {
							if c.nullConds == nil {
								c.nullConds = map[*syntax.WhenCond]bool{}
							}
							c.nullConds[cond] = nt.K == types.Null
						}
					}
					// Later branches see a non-null subject, and this one
					// a null one, of type Nothing?.
					if hasSubj && types.MayBeNull(subjType()) {
						n := narrow{recv: subj.recv, sym: subj.sym, t: subjType().NonNull()}
						later = append(append(later, n), c.impliedBy(n)...)
						if single {
							whenTrue = append(whenTrue, narrow{recv: subj.recv, sym: subj.sym, t: types.NullType})
						}
					}
				}
			default:
				c.value(cond.X, types.BoolType)
				c.wantBool(cond.X)
				if single {
					t, f := c.narrowing(cond.X)
					whenTrue = append(whenTrue, t...)
					later = append(later, f...)
				}
			}
		}
		if hasSubj && !single {
			// is A, is B -> the common supertype of A and B, and, of a
			// sealed type, none of its other subclasses: a when over it
			// in the branch needs only is A and is B.
			n := narrow{recv: subj.recv, sym: subj.sym, t: subjType(), onlyNot: true}
			if t, ok := c.joinIs(br.Conds); ok && !c.subtype(subjType(), t) && c.subtype(t, subjType()) {
				n.t, n.onlyNot = c.isNarrowed(subjType(), t), false
			} else if t, ok := c.joinIsNarrowed(subjType(), br.Conds); ok {
				n.t, n.onlyNot = t, false
			}
			if others := c.sealedOthers(subjType(), br.Conds); others != nil {
				n.not = &coverage{null: true, classes: others}
			}
			if !n.onlyNot || n.not != nil {
				whenTrue = append(whenTrue, n)
			}
		}
		if br.Guard != nil {
			cov = saved
			// The branch isn't taken where its condition fails or its
			// guard does: later branches learn what both say, as K2's
			// smart casts after it. A guard that can't fail leaves the
			// condition's: x is an Int after !is Int if x !== null, for
			// an x that isn't null.
			condFalse := slices.Clone(later[laterLen:])
			later = later[:laterLen]
			switch {
			case x.Subject == nil:
				c.errorf(br.Guard.Pos(), "guard conditions are only allowed in 'when' with a subject")
			case len(br.Conds) > 1:
				c.errorf(br.Guard.Pos(), "a guard condition is not allowed after several conditions separated by commas")
			}
			condTrue := whenTrue
			c.push()
			c.scope.narrows = append(c.scope.narrows, whenTrue...)
			c.value(br.Guard, types.BoolType)
			c.wantBool(br.Guard)
			t, f := c.narrowing(br.Guard)
			c.pop()
			if x.Subject != nil && single && len(condFalse) > 0 {
				if neverFalse(br.Guard) {
					later = append(later, condFalse...)
				} else {
					later = append(later, c.eitherNarrows(condFalse, condTrue, f)...)
				}
			}
			whenTrue = append(whenTrue, t...)
		}
		c.push()
		c.scope.narrows = whenTrue
		exits = append(exits, c.watchExit(vars))
		npending := len(pending)
		bt := c.ifBranch(br.Body, lambdaBranchWant(br.Body, funcBranch, want, valueUsed), valueUsed, sink == nil, &pending)
		funcBranch = firstFuncBranch(funcBranch, bt)
		bts = append(bts, bt)
		c.pop()
		c.pop()
		// A branch whose check is pending, as 2 of null -> 2, has type
		// Nothing until then, but doesn't jump.
		jumped := bt.K == types.Nothing && len(pending) == npending
		anyJump = anyJump || jumped
		if !jumped {
			allJump = false
		}
		if x.Subject != nil && nullFirst && !br.Else {
			if _, null := br.Conds[0].X.(*syntax.NullLit); single && null && br.Guard == nil {
				nullFirst = false
				if jumped {
					if hasSubj && types.MayBeNull(subjType()) {
						n := narrow{recv: subj.recv, sym: subj.sym, t: subjType().NonNull()}
						afterNull = append(append(afterNull, n), c.impliedBy(n)...)
					}
					afterNull = append(afterNull, c.safeReceivers(x.Subject)...)
				}
			} else {
				nullFirst = !matchesNull(br.Conds)
			}
		}
		if !valueUsed {
			unused = c.unusedJoin(unused, br.Body)
		}
		if valueUsed && !softFail {
			if x.Soft {
				if t, ok := c.join(result, bt); ok {
					result = t
				} else {
					softFail = true // a statement, unless exhaustive
				}
				continue
			}
			t, ok := c.joinBranches(result, bt, want, x.P)
			if !ok {
				c.errorf(br.Body.P, "type mismatch: 'when' branches have incompatible types %s and %s", result, bt)
				return types.InvalidType
			}
			result = t
		}
	}
	if hasSubj && x.SubjVar == "" {
		// Nor values the subject is known not to be: E.A after
		// if (e == E.A) return.
		base := subj.sym
		if subj.recv != nil {
			base = subj.recv
		}
		if not := c.scope.excluded(subj.recv, subj.sym); not != nil && (base == nil || c.varUntouched(base)) {
			cov.merge(not)
		}
	}
	covered := x.Subject != nil && c.covers(st, &cov)
	exhaustive := hasElse || covered
	// The subject or the first condition, which is always evaluated,
	// jumps: when (x) { return "" if g -> {} } returns, as in kotlinc.
	jumps := x.Subject != nil && x.Subject.Type().K == types.Nothing
	if len(x.Branches) > 0 && len(x.Branches[0].Conds) > 0 {
		if cx := x.Branches[0].Conds[0].X; cx != nil && cx.Type().K == types.Nothing {
			jumps = true
		}
	}
	if !exhaustive {
		exits = append(exits, c.startExit(vars, later)) // no branch matched
	}
	if x.Soft && (!exhaustive || softFail) {
		// The last statement of a lambda, which then returns Unit. Its
		// branches were checked as values.
		x.Soft = false
		valueUsed = false
		c.completeBranches(pending, result, want, true, anyJump)
		pending = nil
	}
	if c.u.rec != nil {
		c.u.rec.When(x, covered, valueUsed)
	}
	if c.coveredWhens == nil {
		c.coveredWhens = map[*syntax.When]bool{}
	}
	c.coveredWhens[x] = covered
	c.noteRedundantElse(x, st, &cov, hasElse, covered)
	if !exhaustive {
		// As of Kotlin 1.7, a when statement over a Boolean, an enum or a
		// sealed class must be exhaustive too.
		if valueUsed || x.Subject != nil && enumerable(st) {
			c.errorf(x.P, "'when' expression must be exhaustive, add necessary 'else' branch")
			return types.InvalidType
		}
		c.completeBranches(pending, types.NothingType, noWant, false, false)
		if jumps {
			return types.NothingType
		}
		return types.UnitType
	}
	if !hasElse {
		// Exhaustive without else: falling through the branches can't
		// happen, but the JVM needs code there.
		x.NoMatch = "java/lang/IllegalStateException"
		if c.u.ext.kotlin {
			x.NoMatch = "kotlin/NoWhenBranchMatchedException"
		}
	}
	if !valueUsed {
		c.completeBranches(pending, unused, noWant, false, anyJump)
		bodies := make([]*syntax.Block, len(x.Branches))
		for i, br := range x.Branches {
			bodies[i] = br.Body
		}
		c.noteBranchJoin(x, c.statementJoin(bodies...))
		if !allJump {
			allJump = true
			for i, br := range x.Branches {
				allJump = allJump && inferredJump(bts[i], br.Body).K == types.Nothing
			}
		}
		if allJump || jumps {
			return types.NothingType
		}
		return types.UnitType
	}
	if len(x.Branches) == 0 {
		return types.UnitType // when (n) {} of an n: Nothing, as kotlinc types it
	}
	if sink != nil {
		return c.sinkBranches(sink, pending, result)
	}
	result = c.completeBranches(pending, result, want, true, anyJump)
	if want.K == types.Invalid {
		c.noteBranchJoin(x, result)
	}
	return result
}

// joinIs returns the common supertype of the types of a when branch's
// conditions if they are all is T.
func (c *Checker) joinIs(conds []syntax.WhenCond) (types.Type, bool) {
	var t types.Type
	for i, cond := range conds {
		if !cond.Is || cond.Target.K != types.Class {
			return types.InvalidType, false
		}
		if i == 0 {
			t = cond.Target
			continue
		}
		j, ok := c.join(t, cond.Target)
		if !ok || j.K != types.Class {
			return types.InvalidType, false
		}
		t = j
	}
	return t, true
}

// sealedOthers returns, for a subject of sealed type t and when branch
// conditions that are all is checks, the subclasses of t's class that none
// of the checks' classes is or is in: the classes the subject isn't when
// one of the checks holds. It returns nil for other subjects or
// conditions.
func (c *Checker) sealedOthers(t types.Type, conds []syntax.WhenCond) []*types.ClassSym {
	t = t.Flex().NonNull()
	if t.K != types.Class || !t.C.Sealed {
		return nil
	}
	var targets []*types.ClassSym
	for _, cond := range conds {
		if !cond.Is || cond.Target.K != types.Class {
			return nil
		}
		targets = append(targets, cond.Target.C)
	}
	var out []*types.ClassSym
	var walk func(cls *types.ClassSym)
	walk = func(cls *types.ClassSym) {
		for _, s := range cls.Subclasses {
			switch {
			case slices.ContainsFunc(targets, func(k *types.ClassSym) bool { return s.IsSubclassOf(k) }):
			case s.Sealed:
				walk(s)
			case !slices.ContainsFunc(targets, func(k *types.ClassSym) bool { return k.IsSubclassOf(s) }):
				out = append(out, s)
			}
		}
	}
	walk(t.C)
	if out == nil {
		out = []*types.ClassSym{}
	}
	return out
}

// matchesNull reports whether a when branch's conditions on the subject
// may match null: those other than values that aren't null and is T for
// a T that isn't nullable.
func matchesNull(conds []syntax.WhenCond) bool {
	for _, cond := range conds {
		switch {
		case cond.NotIs:
			return true
		case cond.Is:
			if types.MayBeNull(cond.Target) {
				return true
			}
		case cond.In || cond.NotIn:
			return true
		default:
			if !nonNull(cond.X.Type()) {
				return true
			}
		}
	}
	return false
}

func (c *Checker) jump(x *syntax.Jump) {
	switch x.Kind {
	case syntax.RETURN:
		if x.Label != "" && c.labeledReturn(x) {
			return
		}
		switch {
		case c.fn == nil:
			c.errorf(x.P, "'return' is not allowed here")
		case !c.leave(c.base, x):
		case !c.retAllowed && c.exprBodyReturn(x):
		case !c.retAllowed:
			c.errorf(x.P, "'return' needs an explicit return type on '%s'", c.fn.Name)
		case x.X != nil:
			c.assignable(c.returnValue(x.X, c.retType), c.retType, syntax.StartPos(x.X))
		case c.retType.K != types.Unit && c.retType.K != types.Invalid:
			c.errorf(x.P, "this function must return a value of type %s", c.retType)
		}
	case syntax.THROW:
		c.throw(x)
	case syntax.BREAK, syntax.CONTINUE:
		if len(c.loops) == 0 {
			if c.loopsOutside {
				c.errorf(x.P, "'%s' crosses a function boundary", x.Kind)
			} else {
				c.errorf(x.P, "'%s' is not allowed outside a loop", x.Kind)
			}
			return
		}
		loop := c.loops[len(c.loops)-1]
		if x.Label != "" {
			i := len(c.loops) - 1
			for i >= 0 && c.loops[i].label != x.Label {
				i--
			}
			if i < 0 {
				c.errorf(x.P, "unresolved label: %s", x.Label)
				return
			}
			loop = c.loops[i]
		}
		if !c.leave(loop.lambdas, x) {
			return
		}
		x.Loop = loop.node
		c.loopJump(slices.Index(c.loops, loop), x.Kind == syntax.BREAK)
		if x.Kind == syntax.BREAK {
			loop.hasBreak = true
		} else {
			loop.hasContinue = true
		}
	}
}

// entriesQualifier resolves x, the receiver of a member, to an enum's
// nested object named entries, if it is E.entries and E has one: as in
// Kotlin, a qualifier names a class before the enum's entries, which
// E.entries alone is. E.entries.toString() calls the object's.
func (c *Checker) entriesQualifier(x syntax.Expr) {
	m, ok := x.(*syntax.Member)
	if !ok || m.Name != "entries" || m.Object != nil || m.Safe {
		return
	}
	enum := c.classRef(m.X)
	if enum == nil || enum.Entries == nil || c.staticFieldOf(enum, m.Name) != nil {
		return
	}
	if cls := c.classRef(m); cls != nil {
		if obj := objectOf(cls); obj != nil && obj == cls {
			c.checkClassVisible(cls, m.P)
			objectValue(m, obj)
		}
	}
}

// ctorsFit reports whether call x constructs a class cls rather than
// calls one of the functions fs of the same name. If both take the
// arguments, their types decide: Compose's function BorderStroke(Dp,
// Color) next to the constructor BorderStroke(Dp, Brush).
func (c *Checker) ctorsFit(cls *types.ClassSym, fs []*types.FunSym, x *syntax.Call) bool {
	cls.Ensure()
	switch ctors, funs := c.anyMayTake(cls.Ctors, x), c.anyMayTake(fs, x); {
	case !ctors && funs:
		return false
	case ctors && funs:
		ts := c.argTypes(x)
		return c.takeTypes(cls.Ctors, x, ts) || !c.takeTypes(fs, x, ts)
	}
	return true
}

// sinkLambdas sets lambdaSink to on, and returns the function that
// restores it. branchArg is consumed.
func (c *Checker) sinkLambdas(on bool) func() {
	saved := c.lambdaSink
	c.lambdaSink, c.branchArg = on, false
	return func() { c.lambdaSink = saved }
}

// starsBounded returns t, a value's type, with its star arguments of
// out parameters replaced by out projections of their bounds: K2
// captures the stars, so that E of Out<E> <: Out<*> is below the
// capture, which kc approximates by its bound.
func starsBounded(t types.Type) types.Type {
	n := t.NonNull()
	if n.K != types.Class || n.A == nil {
		return t
	}
	args := slices.Clone(n.A.List)
	changed := false
	for i, a := range args {
		if a.V == types.Star && i < len(n.C.TypeParams) && n.C.TypeParams[i].Var == types.Out {
			args[i] = types.Arg{V: types.Out, T: starArgBound(n, i)}
			changed = true
		}
	}
	if !changed {
		return t
	}
	n.A = types.MakeArgs(args)
	if t.N {
		n = n.Nullable()
	}
	return n
}

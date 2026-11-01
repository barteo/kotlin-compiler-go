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
		if want.K != types.Invalid && want.K != types.Unit {
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
	allowRange, sink := c.allowRange, c.sink
	c.allowRange, c.sink = false, nil
	t := c.expr1(x, want, allowRange, sink)
	x.SetType(t)
	c.composeRead(x)
	return t
}

func (c *Checker) expr1(x syntax.Expr, want types.Type, allowRange bool, sink *system) types.Type {
	switch x := x.(type) {
	case *syntax.Ident:
		if x.Object != nil {
			return types.ClassType(x.Object) // an object, already resolved
		}
		if len(c.recvs) > 0 {
			if _, _, local := c.scope.lookup(x.Name); !local {
				if t, ok := c.recvMember(x); ok {
					return t
				}
			}
		}
		sym, t, cast, ok := c.lookupVar(x.Name)
		if ok && sym.Kind == types.Global && (c.cls != nil || c.staticCls != nil) {
			// As in Kotlin, the implicit receivers come before the
			// top-level properties: a property of this, an extension
			// property of this, then a member of an object in scope.
			if t, ok := c.receiverProp(x); ok {
				return t
			}
		}
		if ok {
			c.checkTopVar(sym, x.P)
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
			if x.Name == "Unit" {
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
					return objectValue(x, obj)
				}
			}
			if len(c.topFuns(x.Name)) > 0 {
				c.errorf(x.P, "function invocation '%s()' expected; use ::%s for a reference", x.Name, x.Name)
			} else if cls != nil {
				c.errorf(x.P, "class '%s' cannot be used as a value", x.Name)
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
		return c.objectExpr(x)
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
		return c.binary(x, want, allowRange)
	case *syntax.Member:
		return c.member(x)
	case *syntax.Call:
		if w := want.Flex().NonNull(); x.Literal && w.K == types.Class && w.C.Array && w.C.Elem.IsPrimitive() {
			// [1, 2] where an IntArray is expected: intArrayOf(1, 2).
			name := strings.TrimPrefix(w.C.KotlinName, "kotlin/")
			x.Fun.(*syntax.Ident).Name = strings.ToLower(name[:1]) + name[1:] + "Of"
		}
		return c.call(x, want, sink)
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
			if len(sink.vars) > n {
				sink.fixups = append(sink.fixups, func(s *system) { x.SetType(s.final(t).NonNull()) })
			}
			return t.NonNull()
		}
		t := c.declaredIfNull(x.X, c.value(x.X, xw))
		if t.K == types.Invalid {
			return t
		}
		if k, ok := c.narrowKeyOf(x.X); ok && types.MayBeNull(k.t) {
			// x!! smart-casts x for the code after it.
			k.t = k.t.NonNull()
			c.scope.narrows = append(append(c.scope.narrows, k), c.implied[c.localKey(k)]...)
		}
		return t.NonNull()
	case *syntax.Spread:
		// An argument for a vararg parameter: resolve checks where it goes.
		t := c.value(x.X, want)
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
	return c.objectMember(x)
}

func (c *Checker) lookupVar(name string) (*types.VarSym, types.Type, bool, bool) {
	if sym, depth := c.scope.find(name); sym != nil {
		switch {
		case sym.Delegate != nil:
			c.capture(sym.Delegate, depth) // reading it calls getValue on that
		case sym.Kind == types.BackingField:
			// field is this's field, or a static one: a lambda captures this.
			if p := sym.Prop; p.Kind != types.Global && !p.StaticField() {
				c.captureThis()
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
		if p := c.userProp(c.cls, name); p != nil {
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
		c.unsupported(x.P, "unsigned integers", "unsigned integers need the standard library")
		return types.InvalidType
	}
	if !cls.Value {
		return types.InvalidType
	}
	return types.ClassType(cls)
}

// fitsLit reports whether an integer literal argument can be passed for a
// parameter of type p, once Int doesn't fit: as a Long, Byte or Short.
func fitsLit(lit *syntax.IntLit, p types.Type) bool {
	if lit.Unsigned {
		return false
	}
	switch p.NonNull() {
	case types.LongType, types.ByteType, types.ShortType:
		return intLit(lit, lit.Val, p) == p.NonNull()
	}
	return false
}

func (c *Checker) unary(x *syntax.Unary, want types.Type) types.Type {
	switch x.Op {
	case syntax.MINUS, syntax.PLUS:
		if lit, ok := x.X.(*syntax.IntLit); ok && x.Op == syntax.MINUS {
			// A negative literal, which may be a Byte or Short: -128.
			if w := want.NonNull(); w == types.ByteType || w == types.ShortType {
				t := intLit(lit, -lit.Val, want)
				lit.SetType(t)
				return t
			}
		}
		t := c.value(x.X, want).Flex()
		x.X, t = builtinBound(x.X, t)
		if !builtinOperand(t) {
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
		if t := c.value(x.X, types.BoolType); !builtinOperand(t) {
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
// without a type of their own: 8 * 1024 or -1, but not 1L or 1u.
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
	}
	return false
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
		c.value(x.Y, types.BoolType)
		c.pop()
		c.wantBool(x.Y)
		return types.BoolType

	case syntax.ELVIS:
		// As kotlinc's elvis(x: K?, y: K): K, the left side expects the
		// expected type's nullable: findObject(key) ?: default infers T.
		xw := noWant
		if want.K != types.Invalid && want.K != types.Unit {
			xw = want.Nullable()
		}
		xt := c.safeUnit(x.X, c.value(x.X, xw)) // x?.f() ?: y for a Unit f
		var yt types.Type
		if want.K == types.Invalid && xt.K != types.Invalid {
			// The right side expects the left's type, unless it can't be
			// one: as kotlinc's K is a supertype of both, ktor's
			// kotlinType?.let { serializer(it) } ?: type.serializer() is a
			// KSerializer<Any?> or a KSerializer<T> for a T : Any.
			// A call of a supertype's class expects that supertype:
			// foo ?: listOf() for a MutableList<String>? foo is a
			// List<String>.
			yw := xt.NonNull()
			if cls := c.peekClass(x.Y); cls != nil && yw.K == types.Class && yw.C != cls {
				if sup, ok := c.asSuper(yw, cls); ok {
					yw = sup
				}
			}
			n := len(c.errs)
			yt = c.value(x.Y, yw)
			if len(c.errs) > n && simpleExpr(x.Y) {
				c.errs = c.errs[:n]
				yt = c.value(x.Y, noWant)
			}
		} else {
			yt = c.value(x.Y, want)
		}
		t, ok := c.join(xt.NonNull(), yt)
		if !ok {
			c.errorf(x.P, "type mismatch: incompatible types %s and %s", xt.NonNull(), yt)
			return types.InvalidType
		}
		if yt.K == types.Nothing {
			// x ?: return: the code after it sees x not null.
			c.scope.narrows = append(c.scope.narrows, c.equalNonNull(x.X)...)
		}
		if yt.P {
			// a ?: javaCall() is of a platform type, as the Java value:
			// null if both are, and unchecked where a T? is expected.
			t = t.Platform()
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
		yt := c.value(x.Y, xt)
		x.OpType = c.equality(x.P, x.Op, xt, yt)
		return types.BoolType
	}

	opWant := noWant
	if want.NonNull() == types.LongType && literalArith(x) {
		// Arithmetic of integer literals is a Long where one is
		// expected: val n: Long = 8 * 1024.
		opWant = types.LongType
	}
	xt := c.value(x.X, opWant).Flex()
	x.X, xt = builtinBound(x.X, xt)
	if !builtinOperand(xt) {
		switch x.Op {
		case syntax.PLUS, syntax.MINUS, syntax.STAR, syntax.SLASH, syntax.PERCENT,
			syntax.LT, syntax.GT, syntax.LE, syntax.GE:
			return c.binaryOp(x, xt, want)
		}
	}
	yWant := xt
	switch x.Y.(type) {
	case *syntax.If, *syntax.When, *syntax.Try:
		// An operand's branches have their own types: n - (if (s) 0 else
		// pos) subtracts an Int from a Long.
		yWant = noWant
	}
	yt := c.value(x.Y, yWant).Flex()
	x.Y, yt = builtinBound(x.Y, yt)
	if xt.K == types.Invalid || yt.K == types.Invalid {
		return types.InvalidType
	}
	switch x.Op {
	case syntax.PLUS:
		if xt.NonNull() == types.StringType {
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
		if !ok && !builtinOperand(yt) {
			// An operator extension of a built-in type: 3 * v is
			// 3.times(v).
			x.Y = checked(x.Y, yt)
			return c.binaryOp(x, xt, want)
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
		case xt.IsParam():
			// An operator extension of a type parameter: a context
			// parameter's T.compareTo, as Arrow's x <= y in
			// with(comparator).
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
		// A type parameter can stand for any type, and instances of one
		// class with different type arguments may still be equal.
		return types.AnyType.Nullable()
	}
	if xt.NonNull() != yt.NonNull() {
		// Values of related reference types, such as Any and String, are
		// compared with equals().
		x, y := xt.NonNull(), yt.NonNull()
		if c.subtype(x, y) && !y.IsPrimitive() || c.subtype(y, x) && !x.IsPrimitive() || c.mayBeSame(x, y) {
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
	if x.K != types.Class || y.K != types.Class {
		return false
	}
	if _, ok := c.asSuper(x, y.C); ok {
		return true
	}
	if _, ok := c.asSuper(y, x.C); ok {
		return true
	}
	x.C.Ensure()
	y.C.Ensure()
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

// narrowing returns the smart casts implied by cond being true or false.
func (c *Checker) narrowing(cond syntax.Expr) (whenTrue, whenFalse []narrow) {
	switch x := cond.(type) {
	case *syntax.Binary:
		switch x.Op {
		case syntax.EQ, syntax.NE, syntax.EQ_EQ_EQ, syntax.NOT_EQ_EQ:
			var k narrow
			ok := false
			_, ynull := x.Y.(*syntax.NullLit)
			_, xnull := x.X.(*syntax.NullLit)
			if ynull {
				k, ok = c.narrowKeyOf(x.X)
			} else if xnull {
				k, ok = c.narrowKeyOf(x.Y)
			}
			var n, null []narrow
			if ok && types.MayBeNull(k.t) {
				kn := k
				kn.t = types.NullType
				null = []narrow{kn}
				k.t = k.t.NonNull()
				n = append([]narrow{k}, c.implied[c.localKey(k)]...)
			}
			// a?.b != null, and a?.b == c where c isn't null, mean that a
			// isn't null.
			eq := x.Op == syntax.EQ || x.Op == syntax.EQ_EQ_EQ
			switch {
			case ynull:
				n = append(n, c.safeReceivers(x.X)...)
			case xnull:
				n = append(n, c.safeReceivers(x.Y)...)
			case nonNull(x.Y.Type()):
				n, eq = c.equalNonNull(x.X), !eq
			case nonNull(x.X.Type()):
				n, eq = c.equalNonNull(x.Y), !eq
			}
			whenTrue, whenFalse = null, n
			if !eq {
				whenTrue, whenFalse = n, null
			}
			// a === b, where b is of a class type, means that a is one too:
			// ktor's prev === this.
			if same := c.identNarrows(x); len(same) > 0 {
				if x.Op == syntax.EQ_EQ_EQ {
					whenTrue = append(whenTrue[:len(whenTrue):len(whenTrue)], same...)
				} else {
					whenFalse = append(whenFalse[:len(whenFalse):len(whenFalse)], same...)
				}
			}
			if len(whenTrue) == 0 && len(whenFalse) == 0 {
				return nil, nil
			}
			return whenTrue, whenFalse
		case syntax.AND:
			// The right side ran if a && b is true: its x!! smart-cast x.
			// The right side's smart casts build on the left's: x is Int?
			// && x != null makes x an Int.
			t1, _ := c.narrowing(x.X)
			c.push()
			c.scope.narrows = t1
			t2, _ := c.narrowing(x.Y)
			c.pop()
			return append(append(t1[:len(t1):len(t1)], t2...), c.asserted(x.Y)...), nil
		case syntax.OR:
			_, f1 := c.narrowing(x.X)
			c.push()
			c.scope.narrows = f1
			_, f2 := c.narrowing(x.Y)
			c.pop()
			return nil, append(append(f1[:len(f1):len(f1)], f2...), c.asserted(x.Y)...)
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
		return c.contractNarrows(x, 1), c.contractNarrows(x, 0)
	case *syntax.Ident:
		if x.Member == nil && x.Sym != nil {
			if n, ok := c.conds[x.Sym]; ok {
				return n[0], n[1]
			}
		}
	case *syntax.Is:
		k, ok := c.narrowKeyOf(x.X)
		if !ok || x.Target.K == types.Invalid {
			return nil, nil
		}
		k.t = c.isNarrowed(k.t, x.Target)
		n := []narrow{k}
		if x.Not {
			return nil, n
		}
		return n, nil
	}
	return nil, nil
}

// identNarrows returns the smart casts that a === b or a !== b implies
// for either side, when the other is of a class type that it isn't known
// to be. A side of a generic type keeps it when the other's is generic
// too: kotlinc's smart cast to both would let inference pick either's
// arguments, as bar(s) for s: Slice<X> === SL0: Slice<A>, where kc's
// would replace them.
func (c *Checker) identNarrows(x *syntax.Binary) []narrow {
	if x.Op != syntax.EQ_EQ_EQ && x.Op != syntax.NOT_EQ_EQ {
		return nil
	}
	var out []narrow
	for _, side := range [][2]syntax.Expr{{x.X, x.Y}, {x.Y, x.X}} {
		ot := side[1].Type()
		if ot.K != types.Class || types.MayBeNull(ot) {
			continue
		}
		if k, ok := c.narrowKeyOf(side[0]); ok && !c.subtype(k.t, ot) && (ot.A == nil || k.t.K != types.Class || k.t.A == nil) {
			k.t = c.isNarrowed(k.t, ot)
			out = append(out, k)
		}
	}
	return out
}

// contractNarrows returns the smart casts that the contract of a call's
// function implies when it returns a value (1: true, 0: false, -1: at
// all): the arguments it says aren't null, or are of a type, then.
func (c *Checker) contractNarrows(x *syntax.Call, returns int) []narrow {
	f := x.Sym
	if f == nil || len(f.Implies) == 0 {
		return nil
	}
	var out []narrow
	for _, e := range f.Implies {
		if e.Returns != returns {
			continue
		}
		var arg syntax.Expr
		if e.Param < 0 {
			m, ok := x.Fun.(*syntax.Member)
			if !ok || m.Safe {
				continue
			}
			arg = m.X
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
			whenTrue, _ := c.narrowing(arg) // check(x is T)
			out = append(out, whenTrue...)
			continue
		}
		k, ok := c.narrowKeyOf(arg)
		switch {
		case !ok:
		case e.Is != nil:
			is := *e.Is
			if is.HasParams() && len(x.Reified) == len(f.TypeParams) {
				// The call's type arguments: value is R for its R.
				sub := types.Subst{}
				for i, tp := range f.TypeParams {
					sub[tp] = types.Arg{T: x.Reified[i]}
				}
				is = sub.Apply(is)
			}
			if !is.HasParams() && !c.subtype(k.t, is) {
				k.t = c.isNarrowed(k.t, is)
				out = append(out, k)
			}
		case types.MayBeNull(k.t):
			k.t = k.t.NonNull()
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
	return c.equalNonNull(recv)
}

// equalNonNull returns the smart casts implied by x being equal to a
// value that isn't null: neither x nor the receivers of its safe calls
// are null.
func (c *Checker) equalNonNull(x syntax.Expr) []narrow {
	n := c.safeReceivers(x)
	k, ok := c.narrowKeyOf(x)
	if m, safe := x.(*syntax.Member); safe && m.Safe {
		k, ok = c.memberKey(m, m.Type())
	}
	if ok && types.MayBeNull(k.t) {
		k.t = k.t.NonNull()
		n = append(append(n, k), c.implied[c.localKey(k)]...)
	}
	return n
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

// callReceiver checks the receiver of a member call, expecting want. A
// nullable receiver can be called without ?. if extensions for nullable
// receivers take it, as String?.isNullOrEmpty() does: callReceiver
// returns them, and the receiver's type stays nullable.
func (c *Checker) callReceiver(fn *syntax.Member, want types.Type) (types.Type, []callee, bool) {
	t := c.fixedStub(c.declaredIfNull(fn.X, c.value(fn.X, want)))
	if t.K == types.Invalid {
		return t, nil, false
	}
	t = c.castView(fn.X, t, fn.Name)
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
		c.errorf(fn.P, "only safe (?.) or non-null asserted (!!.) calls are allowed on a nullable receiver of type %s", t)
		return types.InvalidType, nil, false
	}
	return t.NonNull(), nil, true
}

func (c *Checker) member(x *syntax.Member) types.Type {
	if cl, ok := x.X.(*syntax.ClassLit); ok && x.Name == "java" && !x.Safe {
		// X::class.java is the Java class, without a KClass.
		t := c.classLit(cl, true)
		cl.SetType(t)
		return t
	}
	if x.Object != nil {
		return types.ClassType(x.Object) // an object, already resolved
	}
	if cls := c.classRef(x); cls != nil && !c.hasStatic(c.classRef(x.X), x.Name) {
		if obj := objectOf(cls); obj != nil {
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
		return c.superMember(x, s)
	}
	if l, ok := x.X.(*syntax.Lambda); ok && l.Ref != nil && x.Name == "isInitialized" && !x.Safe {
		return c.isInitialized(x, l.Ref)
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
		return xt
	}
	xt = c.castView(x.X, xt, x.Name)
	x.X, xt = builtinBound(x.X, xt)
	if types.MayBeNull(xt) && !xt.P && !x.Safe {
		// Only an extension property for nullable receivers takes it.
		if t, ok := c.extProp(x, xt); ok {
			return t
		}
		c.errorf(x.P, "only safe (?.) or non-null asserted (!!.) calls are allowed on a nullable receiver of type %s", xt)
		return types.InvalidType
	}
	rt := c.boundWith(xt.NonNull(), func(cls *types.ClassSym) bool { return c.hasProp(cls, x.Name) })
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
	case cls != nil && cls.External && cls.Value && cls.Underlying != nil && x.Name == cls.Underlying.Name:
		// A class-path value class's value: the unboxed value itself.
		x.Prop = cls.Underlying
		t = cls.Underlying.Type
	case cls != nil && !cls.External:
		if x.Prop = c.userProp(cls, x.Name); x.Prop == nil {
			t, found = c.javaProperty(x, rt)
			break
		}
		if p := x.Prop; c.privateElsewhere(p) {
			// An extension of the name is read instead, as with calls.
			x.Prop = nil
			if t, found = c.extProp(x, rt); found {
				break
			}
			x.Prop, found = p, true
		}
		ensureVar(x.Prop)
		t = c.memberSubst(rt, x.Prop.Owner).Apply(x.Prop.Type)
		if !x.Safe && c.stableMember(x.Prop) {
			// A smart cast of a.b.
			x.T = t
			if k, ok := c.narrowKeyOf(x); ok && k.t != t {
				x.Cast = needsCast(t, k.t)
				t = k.t
			}
		}
	case cls != nil && c.visibleField(instanceField(cls, x.Name)):
		x.Prop = instanceField(cls, x.Name)
		t = c.memberSubst(rt, x.Prop.Owner).Apply(x.Prop.Type)
	default:
		t, found = c.javaProperty(x, rt)
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
		t = t.Nullable()
	}
	return t
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
	if x.Recv {
		return c.invoke(x, want, sink)
	}
	switch fn := x.Fun.(type) {
	case *syntax.Ident:
		name := fn.Name
		// A variable that can't be invoked doesn't hide functions, as in
		// fun tag(tag: Any?) = tag(Any::class, tag).
		_, t, isVar := c.scope.lookup(name)
		if isVar && c.invokable(t) {
			if !c.localInvokeFits(t, x) && len(c.recvs) > 0 && c.recvCall(x, fn) {
				// One that can't take the arguments gives way to an
				// implicit receiver's function, as in Kotlin, where each
				// level of scope is tried in turn: map(crossinline
				// transform) = transform { value -> ... } calls Flow's
				// transform.
				return c.call(x, want, sink)
			}
			return c.invoke(x, want, sink)
		}
		if fs, depth := c.scope.findFunsFor(name, func(fs []*types.FunSym) bool { return c.anyMayTake(fs, x) }); len(fs) > 0 {
			if slices.ContainsFunc(fs, func(f *types.FunSym) bool { return f.Ext }) {
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
			// a top-level function of the package or one of the class
			// path's, or an extension of this does.
			ms := c.lowPriorityGivesWay(c.accessible(c.javaMethods(c.cls, name)), name, c.cls.This(), x)
			if len(ms) > 0 && (c.anyMayTake(ms, x) ||
				c.objectTaking(name, x) == nil && !c.anyMayTake(c.topFuns(name), x) && !c.anyMayTake(c.kotlinFuns(name, false), x) &&
					!c.thisExtension(name, x)) {
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
				c.captureThis()
				return c.resolve(x, c.memberCallees(ms, c.cls.This()), want, sink)
			}
			if p := c.userProp(c.cls, name); p != nil && !(p.InProgress && c.uninvokable(p, name)) {
				// One being inferred gives way to a function of its name:
				// val navOptions = navOptions { }.
				ensureVar(p)
				if c.invokable(p.Type) {
					return c.invoke(x, want, sink)
				}
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
			if obj := c.objectMethods(name, x); obj != nil {
				// A method of an object in scope: f() is Companion.f().
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
		if mi, ok := c.callableImport(name); ok {
			// An imported static method or method of an object: f() is C.f().
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
			// The program's functions that don't fit the call give way to
			// the class path's, as in Kotlin, where each level of scope is
			// tried in turn: okio's minOf(Long, Int) to the standard
			// library's minOf(Long, Long).
			if !c.libraryCallable(name) {
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
		if name == "String" && len(x.Args) == 0 {
			x.Builtin = types.NewString // String(): a new empty string
			return types.StringType
		}
		if f := arrayFuns[name]; f != nil && (name != "String" || c.mayTake(f, x) || len(c.kotlinFuns(name, false)) == 0) {
			// String(chars); the standard library has String(bytes,
			// charset) and others.
			return c.arrayCall(x, f, want, sink)
		}
		if b, ok := assertions[name]; ok && len(c.funImports[name]) == 0 {
			return c.assertCall(x, b, name, want)
		}
		cls := c.findClass(name)
		var fs []*types.FunSym
		if cls == nil || !constructible(cls) {
			// A function may have a class's name: kotlin.random.Random(seed).
			fs = c.kotlinFuns(name, false)
		} else if gs := c.kotlinFuns(name, false); len(gs) > 0 && cls.External {
			// Or one of the class's: kotlinx.coroutines'
			// CancellationException(message, cause) for the typealias of
			// a class without that constructor.
			cls.Ensure()
			switch ctors, funs := c.anyMayTake(cls.Ctors, x), c.anyMayTake(gs, x); {
			case !ctors && funs:
				fs = gs
			case ctors && funs:
				// The arguments' types decide: Compose's function
				// BorderStroke(Dp, Color) next to the constructor
				// BorderStroke(Dp, Brush).
				ts := c.argTypes(x)
				if !c.takeTypes(cls.Ctors, x, ts) && c.takeTypes(gs, x, ts) {
					fs = gs
				}
			}
		}
		if a := c.findClassifier(name).alias; a != nil && cls != nil && len(fs) == 0 {
			return c.aliasConstruct(x, a, cls, fn.P, want, sink)
		}
		if cls != nil && cls.Object && len(fs) == 0 && c.hasFun(types.ClassType(cls), "invoke", x) {
			// An object with an invoke operator: Colors() is
			// Colors.invoke().
			x.Fun = objectIdent(cls, fn.P)
			return c.invokeValue(x, types.ClassType(cls), want, sink)
		}
		if cls != nil && len(fs) == 0 {
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
		c.errorf(fn.P, "unresolved reference: %s", name)
		c.args(x, nil)
		return types.InvalidType

	case *syntax.Member:
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
		} else if fs := c.libraryPackageFuns(fn); len(fs) > 0 {
			return c.resolve(x, methodCallees(fs, nil), want, sink) // kotlin.runCatching { }
		} else if c.kotlinIntrinsic(fn) {
			// kotlin.arrayOf(x), which arrayCall checks by its simple name.
			id := &syntax.Ident{Name: fn.Name}
			id.P = fn.P
			x.Fun = id
			return c.call(x, want, sink)
		}
		if cls := c.classRef(fn); cls != nil {
			if name := c.qualifiedName(fn); name != "" {
				if a := c.lookupAlias(name); a != nil {
					return c.aliasConstruct(x, a, cls, fn.P, want, sink) // a.b.Alias(1)
				}
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
		if s, ok := fn.X.(*syntax.Super); ok {
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
		if _, ok := c.extFuncVar(fn.Name); ok && !fn.Safe && !c.memberExtFirst(fn, x) {
			// x.f() calls a function value with a receiver: f(x).
			id := &syntax.Ident{Name: fn.Name}
			id.P = fn.P
			x.Fun, x.Args = id, append([]syntax.Expr{fn.X}, x.Args...)
			return c.invoke(x, want, sink)
		}
		rw := noWant
		if types.BitwiseOps[fn.Name] && want.Flex().NonNull() == types.LongType && intLitExpr(fn.X) && !slices.ContainsFunc(x.Args, func(a syntax.Expr) bool { return !intLitExpr(a) }) {
			// As 1 + 2, 1 shl 31 where a Long is expected is a Long's.
			rw = types.LongType
		}
		rt, nullExts, ok := c.callReceiver(fn, rw)
		if !ok {
			c.args(x, nil)
			return types.InvalidType
		}
		if n := c.safeReceivers(x); len(n) > 0 {
			// The arguments are evaluated only if the receiver isn't
			// null: x?.let { f(x) }.
			c.push()
			c.scope.narrows = n
			defer c.pop()
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
		rt = c.boundWith(rt, func(cls *types.ClassSym) bool {
			return len(c.javaMethods(cls, fn.Name)) > 0 || !cls.External && c.userProp(cls, fn.Name) != nil
		})
		cls := c.classOf(rt)
		if cls != nil && !cls.External {
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
				if alias != nil && len(x.TypeArgs) > 0 {
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
			return c.invokeValue(x, fn.T, want, sink)
		}
		switch {
		case nullExts != nil:
			t = c.resolve(x, nullExts, want, sink)
		case cls != nil && !cls.External:
			ms := c.javaMethods(cls, fn.Name)
			if p := c.userProp(cls, fn.Name); len(ms) == 0 && p != nil && !((c.privateElsewhere(p) || !c.invokeFits(p, x)) && c.extCallees(fn.Name, rt) != nil) {
				// A property of function type, called: obj.f(x).
				if fn.Safe {
					return c.safeInvoke(x, fn, want, sink)
				}
				ensureVar(p)
				fn.Prop = p
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
			if x.Infix && !x.Sym.Infix {
				c.errorf(fn.P, "'infix' modifier is required on '%s'", fn.Name)
			}
			if x.Operator && !x.Sym.Operator {
				c.errorf(x.P, "'operator' modifier is required on '%s'", fn.Name)
			}
			if isArrayFactory(x.Builtin) {
				c.reified(x, t)
			}
		}
		if fn.Safe && t.K != types.Unit {
			fn.Nothing = t.K == types.Nothing && !t.N
			t = t.Nullable()
		}
		fn.T = rt
		return t
	}
	return c.invoke(x, want, sink)
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
	other := c.findClass(name) != nil || len(c.topFuns(name)) > 0 || len(c.kotlinFuns(name, false)) > 0
	if v.InProgress {
		// Its own initializer calls the class or function: val Shapes =
		// Shapes(...).
		return other
	}
	ensureVar(v)
	return !c.invokable(v.Type) && other
}

// ctorsTakeOnly reports whether a class's constructors may take call x and
// none of functions fs, of the same name, may.
func (c *Checker) ctorsTakeOnly(cls *types.ClassSym, fs []*types.FunSym, x *syntax.Call) bool {
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

// invokeFitsType reports whether a value of type ft may be invoked with a
// call's arguments, as invokeFits. A value of another type needs an
// invoke operator that may take them: ktor's ByteReadChannel has a
// readBuffer property, a Source, and a readBuffer() extension.
func (c *Checker) invokeFitsType(ft types.Type, x *syntax.Call) bool {
	ft = ft.Flex()
	if !ft.IsFunc() {
		return ft.K == types.Invalid || c.hasFun(ft, "invoke", x)
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

// privateElsewhere reports whether p is a private property of a class
// that the code isn't in, which an extension with its name is called
// instead of: okio's PeekSource(this).buffer() calls Source.buffer(), not
// PeekSource's private buffer. Code in local classes and object
// expressions is taken to see it.
func (c *Checker) privateElsewhere(p *types.VarSym) bool {
	priv := p.Private
	switch d := p.Decl.(type) {
	case *syntax.PropDecl:
		priv = d.Visibility == syntax.Private
	case *syntax.CtorParam:
		priv = d.Visibility == syntax.Private
	}
	if !priv || p.Owner == nil {
		return false
	}
	for k := c.cls; k != nil; k = k.Outer {
		if k == p.Owner || k.Anonymous || k.Local {
			return false
		}
	}
	return true
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
	m := &syntax.Member{X: x.X, Name: name}
	m.P = x.P
	call := &syntax.Call{Fun: m, Args: args}
	call.P = x.P
	if name == "set" {
		call.PassLast() // the value
	}
	x.Call = call
	t := c.expr(call, want)
	if f := call.Sym; f != nil && !f.Operator {
		c.errorf(x.P, "'operator' modifier is required on '%s' in '%s'", name, f.Owner.DisplayName())
	}
	return t
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

func (c *Checker) builtinMember(x *syntax.Call, fn *syntax.Member, rt types.Type, want types.Type, sink *system) types.Type {
	if n, ok := types.OperatorFuns[fn.Name]; ok && (rt.IsNumeric() || rt == types.CharType) && len(x.Args) == n && !c.extShadows(fn.Name, rt, x) {
		// An operator function called by name, 1.plus(2), is the
		// operator: 1 + 2.
		c.noSpread(x)
		at := types.InvalidType
		if n == 1 {
			at = c.value(x.Args[0], noWant).Flex()
		}
		if _, res, ok := types.OperatorFunType(fn.Name, rt, at); ok {
			x.Builtin = types.OperatorFun
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
		if cands := c.extCalleesFor(fn.Name, rt, x); len(cands) > 0 {
			return c.resolve(x, cands, want, sink)
		}
		if object != nil && anyMember[fn.Name] {
			// Any's members, of a number or a function value: boxed,
			// so that 0.0.equals(-0.0) is false, as in Kotlin.
			return c.resolve(x, c.memberCallees(c.javaMethods(object, fn.Name), types.AnyType), want, sink)
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
	return s.ret
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

func (c *Checker) ifExpr(x *syntax.If, want types.Type, valueUsed bool) types.Type {
	c.value(x.Cond, types.BoolType)
	c.wantBool(x.Cond)
	whenTrue, whenFalse := c.narrowing(x.Cond)

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
	defer func() {
		if len(vars) > 0 {
			c.merge(vars, exits)
		}
	}()
	var pending []pendingBranch
	c.push()
	c.scope.narrows = whenTrue
	exits = append(exits, c.watchExit(vars))
	thenT := c.branch(x.Then, want, valueUsed, &pending)
	c.pop()
	if x.Else == nil && len(vars) > 0 {
		exits = append(exits, c.startExit(vars, whenFalse))
	}
	if x.Else == nil {
		if valueUsed {
			c.errorf(x.P, "'if' must have both main and 'else' branches if used as an expression")
			return types.InvalidType
		}
		c.completeBranches(pending, types.NothingType, noWant, false)
		return types.UnitType
	}
	c.push()
	c.scope.narrows = whenFalse
	exits = append(exits, c.watchExit(vars))
	elseWant := want
	if l := blockLambda(x.Else); l != nil && valueUsed && want.K == types.Invalid && thenT.Flex().IsFunc() && !thenT.HasVars() && (len(l.Params) > 0 || !noInputs(thenT.Flex())) {
		// A lambda's parameters are the other branch's function type's:
		// if (c) consumer else { node -> put(node) }. One without them
		// has its own type, which the branches' type is above:
		// if (c) { { 1 } } else { { 1f } }.
		elseWant = thenT
	}
	elseT := c.branch(x.Else, elseWant, valueUsed, &pending)
	c.pop()
	if !valueUsed {
		c.completeBranches(pending, types.NothingType, noWant, false)
		if thenT.K == types.Nothing && elseT.K == types.Nothing {
			return types.NothingType
		}
		return types.UnitType
	}
	t, ok := c.joinBranch(thenT, elseT, want, x.Else.P)
	if !ok {
		c.errorf(x.P, "type mismatch: 'if' branches have incompatible types %s and %s", thenT, elseT)
		return types.InvalidType
	}
	return c.completeBranches(pending, t, want, true)
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
	// open is set when the call needs the other branches' types: it has
	// no lambdas, or they don't give their parameter types. See openLast.
	open bool
	// unused is set when the branch's value isn't used: its block's type
	// stays the statement's.
	unused bool
	// scope is the branch's block's, with its locals and smart casts,
	// which its lambdas see.
	scope *scope
}

// branch checks a branch of an if or when. Without a known expected
// type, or with Any, a branch that is a single generic call is left
// incomplete if it can't be inferred on its own: Kotlin infers the
// branches together, so the others' types complete it, as Success's in
// if (c) closed(cause) else success(x) for an Any?, and build {}'s in
// if (c) build { yield(x) } else build {}, also unused. It is then seen as
// Nothing, which the join ignores, until completeBranches.
func (c *Checker) branch(b *syntax.Block, want types.Type, valueUsed bool, pending *[]pendingBranch) types.Type {
	if want.K != types.Invalid && want.K != types.Any && !want.HasVars() || len(b.Stmts) == 0 {
		return c.block(b, want, valueUsed)
	}
	es, ok := b.Stmts[len(b.Stmts)-1].(*syntax.ExprStmt)
	if !ok {
		return c.block(b, want, valueUsed)
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
	if len(inner.lambdas) == 0 && inner.selfInferable(t) {
		inner.finish()
		if !valueUsed {
			return bt
		}
		t = inner.final(t)
		b.T = t
		return t
	}
	*pending = append(*pending, pendingBranch{sys: inner, t: t, body: b, scope: c.blockScope, unused: !valueUsed})
	if !valueUsed {
		return bt
	}
	return types.NothingType
}

// completeBranches completes the pending branches with the type of the
// others, result, or the expected type, and returns the type of all. If
// the value isn't used, only the open ones take the others' type.
func (c *Checker) completeBranches(pending []pendingBranch, result, want types.Type, valueUsed bool) types.Type {
	c.openLast(pending)
	for _, p := range pending {
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
		if target.K == types.Nothing || target.K == types.Invalid || target.HasVars() {
			target = want
		}
		if target.K != types.Invalid && !target.HasVars() && (valueUsed || p.open) {
			trial := p.sys.clone()
			trial.sub(p.t, target)
			if trial.ok {
				p.sys = trial
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
		bt := p.sys.final(p.t)
		if !p.unused {
			p.body.T = bt
		}
		if j, ok := c.join(result, bt); ok {
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
			c.stubInference(p.sys, pl.l, params, ft)
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
	var pending []pendingBranch
	softFail := false
	// As after an if, the vars the branches assign have the join of their
	// types: merged in the enclosing scope, after the pop below.
	vars := c.watchedVars(x)
	var exits []*branchExit
	defer func() {
		if len(vars) > 0 {
			c.merge(vars, exits)
		}
	}()
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
			if types.MayBeNull(st) {
				c.implyNonNull(x.SubjSym, x.Subject)
			}
			subj, hasSubj = narrow{sym: x.SubjSym, t: st}, true
		} else {
			st = c.value(x.Subject, noWant)
			x.SubjSym = &types.VarSym{Name: "<subject>", Type: st, Kind: types.Local}
			subj, hasSubj = c.narrowKeyOf(x.Subject)
		}
	}
	hasElse := false
	var cov coverage
	result := types.NothingType
	allJump := true
	var later []narrow // what the failed conditions of the branches so far imply
	nullFirst := true  // no branch so far matches null
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
		for j := range br.Conds {
			cond := &br.Conds[j]
			switch {
			case cond.Is || cond.NotIs:
				if x.Subject == nil {
					c.errorf(cond.P, "'is' conditions need a 'when' subject")
					continue
				}
				cond.Target = c.checkTarget(cond.Type, st, types.InvalidType, true)
				if cond.Is && cond.Target.K == types.Class {
					cov.classes = append(cov.classes, cond.Target.C)
				}
				if hasSubj && single && cond.Target.K != types.Invalid {
					n := narrow{recv: subj.recv, sym: subj.sym, t: c.isNarrowed(subjType(), cond.Target)}
					if cond.Is {
						whenTrue = append(whenTrue, n)
					} else {
						later = append(later, n)
					}
				}
			case cond.In || cond.NotIn:
				if x.Subject == nil {
					c.errorf(cond.X.Pos(), "'in' conditions need a 'when' subject")
					c.value(cond.X, noWant)
					break
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
				if !(isIntOrLong(st) && isIntOrLong(rt.RangeElem())) {
					c.wantType(x.Subject, st, rt.RangeElem())
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
					cond.OpType = c.equality(cond.X.Pos(), syntax.EQ, st, ct)
				}
				cov.add(cond.X)
				if single && nonNull(ct) {
					// Equal to a value that isn't null: neither the
					// subject nor the receivers of its safe calls are.
					if hasSubj && types.MayBeNull(subjType()) {
						n := narrow{recv: subj.recv, sym: subj.sym, t: subjType().NonNull()}
						whenTrue = append(append(whenTrue, n), c.implied[c.localKey(n)]...)
					}
					whenTrue = append(whenTrue, c.safeReceivers(x.Subject)...)
				}
				if _, ok := cond.X.(*syntax.NullLit); ok {
					// Later branches see a non-null subject, and this one
					// a null one, of type Nothing?.
					if hasSubj && types.MayBeNull(subjType()) {
						later = append(later, narrow{recv: subj.recv, sym: subj.sym, t: subjType().NonNull()})
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
			// is A, is B -> the common supertype of A and B.
			if t, ok := c.joinIs(br.Conds); ok && !c.subtype(subjType(), t) && c.subtype(t, subjType()) {
				whenTrue = append(whenTrue, narrow{recv: subj.recv, sym: subj.sym, t: c.isNarrowed(subjType(), t)})
			}
		}
		if br.Guard != nil {
			cov = saved
			later = later[:laterLen]
			if x.Subject == nil {
				c.errorf(br.Guard.Pos(), "guard conditions are only allowed in 'when' with a subject")
			}
			c.push()
			c.scope.narrows = append(c.scope.narrows, whenTrue...)
			c.value(br.Guard, types.BoolType)
			c.wantBool(br.Guard)
			t, _ := c.narrowing(br.Guard)
			c.pop()
			whenTrue = append(whenTrue, t...)
		}
		c.push()
		c.scope.narrows = whenTrue
		exits = append(exits, c.watchExit(vars))
		bt := c.branch(br.Body, want, valueUsed, &pending)
		c.pop()
		c.pop()
		if bt.K != types.Nothing {
			allJump = false
		}
		if x.Subject != nil && nullFirst && !br.Else {
			if _, null := br.Conds[0].X.(*syntax.NullLit); single && null && br.Guard == nil {
				nullFirst = false
				if bt.K == types.Nothing {
					if hasSubj && types.MayBeNull(subjType()) {
						afterNull = append(afterNull, narrow{recv: subj.recv, sym: subj.sym, t: subjType().NonNull()})
					}
					afterNull = append(afterNull, c.safeReceivers(x.Subject)...)
				}
			} else {
				nullFirst = !matchesNull(br.Conds)
			}
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
			t, ok := c.joinBranch(result, bt, want, br.Body.P)
			if !ok {
				c.errorf(br.Body.P, "type mismatch: 'when' branches have incompatible types %s and %s", result, bt)
				return types.InvalidType
			}
			result = t
		}
	}
	exhaustive := hasElse || x.Subject != nil && c.covers(st, &cov)
	if !exhaustive && len(vars) > 0 {
		exits = append(exits, c.startExit(vars, nil)) // no branch matched
	}
	if x.Soft && (!exhaustive || softFail) {
		// The last statement of a lambda, which then returns Unit. Its
		// branches were checked as values.
		x.Soft = false
		valueUsed = false
		c.completeBranches(pending, result, want, true)
		pending = nil
	}
	if !exhaustive {
		// As of Kotlin 1.7, a when statement over a Boolean, an enum or a
		// sealed class must be exhaustive too.
		if valueUsed || x.Subject != nil && enumerable(st) {
			c.errorf(x.P, "'when' expression must be exhaustive, add necessary 'else' branch")
			return types.InvalidType
		}
		c.completeBranches(pending, types.NothingType, noWant, false)
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
		c.completeBranches(pending, types.NothingType, noWant, false)
		if allJump {
			return types.NothingType
		}
		return types.UnitType
	}
	return c.completeBranches(pending, result, want, true)
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
		case !c.retAllowed:
			c.errorf(x.P, "'return' needs an explicit return type on '%s'", c.fn.Name)
		case x.X != nil:
			c.assignable(c.value(x.X, c.retType), c.retType, x.X.Pos())
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
		if x.Kind == syntax.BREAK {
			loop.hasBreak = true
		}
	}
}

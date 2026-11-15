// A Java annotation's elements without a default need arguments, its
// value array excepted; and no positional argument follows a named one,
// as Java's elements have no order.

@A("1", "2", y = 1) fun ok1() {}
@A(y = 2) fun ok2() {}
@D fun ok3() {}
@D(value = ["1"], y = 3) fun ok4() {}
@A("8", "9") fun missing() {} // ERROR no value passed for parameter 'y'
@D(y = 3, "7") fun mixed() {} // ERROR mixing named and positional

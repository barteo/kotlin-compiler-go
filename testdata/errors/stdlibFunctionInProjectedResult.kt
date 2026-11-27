// A lambda for a function type whose result is an in projection, which
// conflicts with its variance, returns the projection's type, as in K2;
// one for a star returns anything.

var f12: Function0<in Int> = { -> 1 } // ERROR projection is conflicting with variance
var f13: Function0<*> = { -> "" }

fun assign() {
    f12 = { -> "" } // ERROR type mismatch: inferred type is String but Int was expected
    f12 = { -> 2 }
    f13 = { -> "" }
}

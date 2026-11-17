// A function value isn't converted to the fun interface or Java interface
// that a parameter of the callee's own type parameter's type is given, by
// its type arguments or the expected type. It is to one a receiver's type
// or a typealias gives, and when the call that infers the type parameter
// is itself passed for a parameter of a fun interface type.
fun interface KF { fun run() }
fun <R> kg(r: R) {}
fun <R> kn(r: R?) {}
fun <R> kv(vararg r: R) {}
fun <R> kid(r: R): R = r
fun consumeKF(k: KF?) {}
class HoldKF(k: KF?)
class KC<K> { fun f(k: K) {} }
class Box<T>(t: T)
class Two<A, B>(a: A, b: B)
fun <T> KC<T>.ext(t: T) {}
typealias RBox = Box<Runnable>
typealias TA<K> = Box<K>
typealias TI<K> = Two<Runnable, K>
typealias TN = TA<Runnable>

fun own(fv: () -> Unit) {
    C.g<Runnable> { } // ERROR type mismatch: inferred type is () -> Unit but Runnable! was expected
    C.g<KF> { } // ERROR type mismatch
    kg<Runnable> { } // ERROR type mismatch
    kg<KF> { } // ERROR type mismatch
    kn<Runnable> { } // ERROR type mismatch
    kv<Runnable>({ }) // ERROR type mismatch
    kg<Runnable>(fv) // ERROR type mismatch
    C.g<Runnable>(fun() {}) // ERROR type mismatch
    val r: Runnable = C.id<Runnable> { } // ERROR type mismatch
    Box<Runnable> { } // ERROR type mismatch
    val b: Box<Runnable> = Box { } // ERROR type mismatch
    KC<Runnable>().ext { } // ERROR type mismatch
    TA<Runnable> { } // ERROR type mismatch
    TI<Runnable>({ }, { }) // ERROR type mismatch
    consumeKF(kid(kid { })) // ERROR type mismatch
    consumeKF(kid<KF> { }) // ERROR type mismatch
    val k: KF = kid { } // ERROR type mismatch
}

fun given(c: C<Runnable>, k: KC<KF>, fv: () -> Unit) {
    c.f { }
    c.f(fv)
    C<Runnable>().f { }
    k.f { }
    KC<Runnable>().f { }
    C.g<Runnable>(Runnable { })
    kg<KF>(KF { })
    RBox { }
    TI<Int>({ }, 1)
    TN { }
    consumeKF(kid { })
    HoldKF(kid { })
}

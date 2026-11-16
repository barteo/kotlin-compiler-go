// A conflicting in projection of Java's covariant Iterable or Iterator
// is a star; a callable reference to a static member of a generic Java
// class takes its arguments; and an instance method whose last
// parameter is a Continuation is a suspend function.

fun f(x: (Int) -> String) {}

fun use(x: List<String>, y: List<*>, w: MutableList<in CharSequence>) {
    A.foo(x)
    A.foo(y)
    A.foo(w)
    A.bar(x.iterator())
    A.bar(w.iterator())
    f(A<*>::baz)
}

suspend fun <T : Any> susp(a: A<String>, p: T, c: kotlin.coroutines.Continuation<String>): T {
    a.raw()
    A.notSuspend(c)
    return a.susp(p)
}

fun notSuspending(a: A<String>, c: kotlin.coroutines.Continuation<String>) {
    a.raw(c) // ERROR takes 1 arguments
}

inline fun takesInline(f: () -> Unit) = f()
inline fun takesNoinline(noinline f: () -> Unit) = f
inline fun takesVararg(vararg fs: () -> Unit) = fs.size
fun plain(f: () -> Unit) = f()
open class Holder(val f: () -> Unit)

inline fun allowed(s: () -> Unit, ext: Int.(Int) -> Unit, noinline n: () -> Unit) {
    s()
    s.invoke()
    (l@ s)()
    1.ext(2)
    ext(1, 2)
    takesInline(s)
    takesInline(f = s)
    takesInline(l@ s)
    val v = n
    plain(n)
}

inline fun stored(s: () -> Unit) {
    val v = s // ERROR illegal usage of inline-parameter 's' in 'stored'
}

inline fun returned(s: () -> Unit): () -> Unit = s // ERROR illegal usage of inline-parameter 's'

inline fun compared(s: () -> Unit) = s == s // ERROR illegal usage of inline-parameter 's'

inline fun passed(s: () -> Unit) {
    plain(s) // ERROR illegal usage of inline-parameter 's'
    takesNoinline(s) // ERROR illegal usage of inline-parameter 's'
    takesVararg(s) // ERROR illegal usage of inline-parameter 's'
    Holder(s) // ERROR illegal usage of inline-parameter 's'
    val call = { g: () -> Unit -> g() }
    call(s) // ERROR illegal usage of inline-parameter 's'
}

inline fun inLambda(s: () -> Unit) {
    { val v = s }() // ERROR illegal usage of inline-parameter 's'
}

inline fun inObject(crossinline s: () -> Unit) {
    object : Holder(s) {} // ERROR illegal usage of inline-parameter 's'
}

inline fun reference(s: (Int) -> Int) = s::invoke // ERROR illegal usage of inline-parameter 's'

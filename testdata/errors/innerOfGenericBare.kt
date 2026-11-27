// An inner class of a generic class written without type arguments after
// is or as takes them from the subject's type, as a bare type; and a
// member that breaks its class's variance is reported, but public still.
class Outer<E> {
    inner open class InnerBase<F>
    inner class Inner<H> : InnerBase<H>() {
        val prop: E = null!!
    }

    fun foo(x: InnerBase<String>, y: Any?): E? {
        if (x is Inner) {
            return x.prop
        }
        if (y is Inner) return null // ERROR Inner
        return null
    }
}

class In<in T> {
    fun accept(t: Inner) {} // ERROR declared as 'in'
    inner class Inner
}

fun use(i: In<String>, n: In<String>.Inner) {
    i.accept(n)
}

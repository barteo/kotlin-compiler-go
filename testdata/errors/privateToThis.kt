// A private member of a class with in or out type parameters whose
// signature breaks their variance may be used only on the class's own
// this.

class Producer<out T>(t: T) {
    private var t: T = t
    private fun put(x: T) {
        t = x
    }
    private val ok = 1

    fun own(x: @UnsafeVariance T) {
        put(x)
        this.put(x)
        this@Producer.t = x
        val r = this::t
        val o = object {
            fun f() = t
        }
    }

    fun other(p: Producer<@UnsafeVariance T>, x: @UnsafeVariance T) {
        p.t // ERROR cannot access 't': it is private to this in 'Producer'
        p.put(x) // ERROR cannot access 'put': it is private to this in 'Producer'
        p.t = x // ERROR cannot access 't': it is private to this in 'Producer'
        val r = p::t // ERROR cannot access 't': it is private to this in 'Producer'
        p.ok
        with(p) {
            put(x) // ERROR cannot access 'put': it is private to this in 'Producer'
        }
    }
}

fun <R> with(r: R, f: R.() -> Unit) = r.f()

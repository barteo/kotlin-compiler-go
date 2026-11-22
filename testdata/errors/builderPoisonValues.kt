class Ctl<T> {
    fun put(t: T) {}
    fun get(): T = TODO()
    val size: Int = 0
    val first: T get() = TODO()
    fun count(): Int = 0
    fun same(): Ctl<T> = this
}

fun <S> gen(g: Ctl<S>.() -> Unit): S = TODO()
fun <X> X.app(f: X.() -> Unit): X = this
fun tk(i: Int) {}
fun tkAny(i: Any?) {}
fun tkS(i: String) {}

// As K2, after the call that poisons a builder's lambda (see
// stdlibBuilderPoisonLambda.kt), a call that takes a member of the
// builder's receiver, or a local of the builder's type, is checked with
// the builder's system, and its argument is a mismatch.
fun values(k: Int) {
    gen {
        put("")
        val before = get()
        Unknown234(1).app { } // ERROR unresolved reference: Unknown234
        tk(size) // ERROR argument type mismatch
        tk(this.size) // ERROR argument type mismatch
        tkAny(first) // ERROR argument type mismatch
        tk(count()) // ERROR argument type mismatch
        tkAny(same()) // ERROR argument type mismatch
        tk(k + size) // ERROR argument type mismatch
        val g = get()
        tkAny(g) // ERROR argument type mismatch
        tkS(g) // ERROR argument type mismatch
        tkAny(g.length) // ERROR argument type mismatch
        tkAny("$g") // ERROR argument type mismatch
        tkS(before) // ERROR argument type mismatch
        // Not a local with a type of its own.
        val h: String = get()
        tkS(h)
        val n = count()
        tk(n)
        val z = size
        tk(z)
        // Not without a call that takes them.
        first
        get()
        g.length
    }
}

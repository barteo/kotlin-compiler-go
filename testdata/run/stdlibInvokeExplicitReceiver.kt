// A property of a function type with a receiver, called on a written
// receiver, x.bar(), is the bar in scope invoked with x when its own
// call would need an implicit receiver: with(b) { b2.bar() } is b's bar
// with b2. See errors/stdlibInvokeExplicitReceiver.kt.
class B(val n: String) {
    val bar: B.() -> String get() = { "getter on ${this@B.n}, receiver ${this.n}" }
    fun inB(o: B) = o.bar()
}
class A { val foo: B.() -> String get() = { "foo on $n" } }
fun main() {
    val b = B("b"); val b2 = B("b2")
    println(with(b) { b2.bar() })
    println(with(b) { this.bar() })
    println(with(b) { bar() })
    println(b.inB(b2))
    println(with(b) { A().foo(this) })
    println(with(b) { (A().foo)() })
}

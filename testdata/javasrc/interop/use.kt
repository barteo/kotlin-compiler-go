package interop

fun interopDemo() {
    println(FromJava().size2)
    val sized: Sized = FromJava()
    println(sized.size2)
    println(JavaDerived().label)
    println(JavaGetters().title)
    println(Tally().count)
    val counted: JavaCounted = Tally()
    println(counted.count)
    val d: Described = Description()
    println("${Description().text} ${d.text}")
    val outer = Outer("o")
    val inner: Outer<String>.Inner<Int> = outer.Inner<Int>(3)
    println("${inner.value} ${inner.extra}")
    println(Outer(5).Plain().describe())
    println(Outer<String>().Plain().describe())
    val o = Overloads()
    println(o.foo({ "got $it" }, ""))
    println(o.bar { "got $it" })
    println(o.test { })
    println(samOverload { "generic" })
    nothingFromJava(object : Nulls.Visitor<Nothing> {})
    println(StaticOuter().Inner().test())
}

// foo() in an inner subclass of a Java class is its static foo(), which
// comes before the outer class's member foo().
class StaticOuter {
    fun foo() = "outer foo"
    inner class Inner : StaticBase() {
        fun test() = foo()
    }
}

// T of a Java <T> T set(String, T), for a null, is Nothing!, which takes
// null and returns: the code after the call runs.
fun nothingFromJava(v: Nulls.Visitor<Nothing>) {
    val n = Nulls()
    n.set("key", null)
    println("after set")
    val s: String? = n.set("key", null)
    println(s)
    println(n.accept(v, null) == null)
    println(n.remap<Int, String>(2) { k, v -> null })
}

fun <T> samOverload(action: () -> T): T = action()
fun samOverload(action: Runnable): String = "runnable"


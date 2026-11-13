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
}

fun <T> samOverload(action: () -> T): T = action()
fun samOverload(action: Runnable): String = "runnable"


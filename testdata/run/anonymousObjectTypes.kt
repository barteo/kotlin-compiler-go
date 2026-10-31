// Object expressions keep their anonymous types on private declarations
// and on members of local and anonymous classes, so their members are
// accessible through them.

fun <T> run(f: () -> T) = f()

class C {
    val s = "c"
    fun before() = o.bar()
    private val o = object { fun bar() = s + "!" }
    fun inLambda() = run { o.bar() }
}

private fun mk() = object { fun qux() = "qux"; val n = 3 }

private val top = object { var count = 0 }

fun nested(): String {
    val boo = "baz"
    val foo = object { val bar = object { val baz = boo } }
    return foo.bar.baz
}

fun inLocal(): Int {
    class L { fun make() = object { val x = 42 } }
    return L().make().x
}

fun main() {
    println(C().before())
    println(C().inLambda())
    println(mk().qux() + mk().n)
    top.count++
    top.count += 2
    println(top.count)
    println(nested())
    println(inLocal())
}

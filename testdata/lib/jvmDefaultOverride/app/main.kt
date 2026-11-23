// kotlinc: -jvm-default=enable
// An override with the type argument in place has the descriptor that the
// inherited member has there: the interface, the interfaces under it and
// the classes that implement them have the one method, the override's.
interface Foo2 : Foo<String> {
    override fun test(p: String): String = p
    override val String.prop: String
        get() = this
}

interface Foo3 : Foo<String>, Foo2
interface Foo4 : Foo2, Foo<String>

class Base : Foo3
class Base4 : Foo4
class Direct : Foo<String>, Foo2

fun members(name: String) {
    val c = Class.forName(name)
    for (m in c.declaredMethods.sortedBy { it.name + it.parameterTypes.joinToString { p -> p.name } }) {
        println(name + "." + m.name + "(" + m.parameterTypes.joinToString { p -> p.simpleName } + ")" + (if (m.isSynthetic) " synthetic" else ""))
    }
}

fun main() {
    for (f in listOf<Foo<String>>(Base(), Base4(), Direct())) {
        println(f.test("O") + with(f) { "K".prop } + " " + f.keep("x"))
    }
    val base = Base()
    println(base.test("O") + with(base) { "K".prop })
    for (name in listOf("Foo2", "Foo3", "Foo4")) {
        members(name)
        members(name + "\$DefaultImpls")
        val impls = Class.forName(name + "\$DefaultImpls")
        val receiver = if (name == "Foo4") Base4() else Base()
        println(impls.getMethod("test", Class.forName(name), String::class.java).invoke(null, receiver, "ok"))
        println(impls.getMethod("keep", Class.forName(name), String::class.java).invoke(null, receiver, "ok"))
    }
    members("Base")
    members("Direct")
}

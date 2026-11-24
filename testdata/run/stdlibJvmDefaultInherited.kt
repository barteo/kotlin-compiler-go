// kotlinc: -jvm-default=enable
// With default methods and DefaultImpls classes beside them: the
// DefaultImpls methods of an interface for the bodies it inherits
// take the member's parameters as the interface sees them, with the type
// arguments of its supertype, and include member extensions, which the
// classes that implement the interface get too.

class Fin(val n: Int)

interface Cond<T> {
    fun test(t: T): Boolean = t != null
    fun same(t: T, times: Int = 1): T = t
    fun first(): T? = null
    fun List<T>.count2(): Int = size * 2
    fun T.tag(): String = "tag " + this
    val T.twice: String get() = "" + this + this
}

interface FinCond : Cond<Fin>
interface IntCond : Cond<Int>
interface Lists<U> : Cond<List<U>>
interface StringLists : Lists<String>

interface Base { fun String.ext(): Int = length }
interface Sub : Base
interface SubSub : Sub { fun Int.ext(): Int = this + 1 }

// An override with the type argument in place, which the interfaces
// under it and their classes have instead of the inherited body.
interface Named<T> {
    fun name(t: T): String = "inherited"
    val T.label: String get() = "inherited"
    fun keep(t: T): String = "keep " + t
}

interface StringNamed : Named<String> {
    override fun name(t: String): String = t
    override val String.label: String get() = this
}

interface Both : Named<String>, StringNamed
interface Both2 : StringNamed, Named<String>
interface AbstractNamed : Named<String> { override fun name(t: String): String }

class BothImpl : Both
class Both2Impl : Both2
class DirectImpl : Named<String>, StringNamed
class AbstractImpl : AbstractNamed { override fun name(t: String): String = "own " + t }

class FinImpl : FinCond
class IntImpl : IntCond
class StringImpl : StringLists
class Impl : Sub
class Impl2 : SubSub
class Own : Sub { override fun String.ext(): Int = -1 }

fun <T> viaCond(c: Cond<T>, t: T): String = with(c) { "" + test(t) + " " + same(t) + " " + first() + " " + listOf(t, t).count2() + " " + t.tag() + " " + t.twice }

fun members(name: String) {
    val c = Class.forName(name)
    for (m in c.declaredMethods.sortedBy { it.name + it.parameterTypes.joinToString { p -> p.name } }) {
        println(name + "." + m.name + "(" + m.parameterTypes.joinToString { p -> p.simpleName } + "): " + m.returnType.simpleName + (if (m.isSynthetic) " synthetic" else ""))
    }
}

fun main() {
    members("FinCond\$DefaultImpls")
    members("IntCond\$DefaultImpls")
    members("StringLists\$DefaultImpls")
    members("Sub\$DefaultImpls")
    members("SubSub\$DefaultImpls")
    members("Impl")

    for (n in listOf<Named<String>>(BothImpl(), Both2Impl(), DirectImpl(), AbstractImpl())) {
        println(n.name("a") + " " + with(n) { "b".label } + " " + n.keep("c"))
    }
    println(BothImpl().name("d") + " " + with(Both2Impl()) { "e".label })
    for (name in listOf("StringNamed", "Both", "Both2", "AbstractNamed")) {
        members(name + "\$DefaultImpls")
    }
    println(Class.forName("Both\$DefaultImpls").getMethod("name", Both::class.java, String::class.java).invoke(null, BothImpl(), "f"))
    println(Class.forName("Both2\$DefaultImpls").getMethod("name", Both2::class.java, String::class.java).invoke(null, Both2Impl(), "g"))

    val fin = Fin(4)
    val impls = Class.forName("FinCond\$DefaultImpls")
    println(impls.getMethod("test", FinCond::class.java, Fin::class.java).invoke(null, FinImpl(), fin))
    println((impls.getMethod("same", FinCond::class.java, Fin::class.java, Int::class.java).invoke(null, FinImpl(), fin, 1) as Fin).n)
    println(impls.getMethod("tag", FinCond::class.java, Fin::class.java).invoke(null, FinImpl(), fin).toString().startsWith("tag Fin"))
    val ints = Class.forName("IntCond\$DefaultImpls")
    println(ints.getMethod("same", IntCond::class.java, Int::class.java, Int::class.java).invoke(null, IntImpl(), 7, 1))
    println(ints.getMethod("tag", IntCond::class.java, Int::class.java).invoke(null, IntImpl(), 8))
    println(ints.getMethod("first", IntCond::class.java).invoke(null, IntImpl()))
    println(Class.forName("Sub\$DefaultImpls").getMethod("ext", Sub::class.java, String::class.java).invoke(null, Impl(), "abcd"))

    println(viaCond(IntImpl(), 3))
    println(viaCond(StringImpl(), listOf("a")))
    println(FinImpl().test(fin))
    println(FinImpl().same(fin, 2).n)
    with(Impl()) { println("abc".ext()) }
    with(Impl2()) { println("abcde".ext() + 1.ext()) }
    with(Own()) { println("abc".ext()) }
    val b: Base = Impl()
    with(b) { println("ab".ext()) }
}

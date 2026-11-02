// Classes whose members kc must link as kotlinc does: a local data
// class's copy passes its captures, a data class inherits a superclass's
// final toString, hashCode and equals, super.clone() calls Object's, an
// annotation class's companion is in a static field, and an enum entry's
// inner class reads the entry being constructed.

abstract class FinalMembers {
    final override fun toString() = "final toString"
    final override fun hashCode() = 42
    final override fun equals(other: Any?) = false
}

data class InheritsFinal(val field: String) : FinalMembers()

open class FinalToString {
    final override fun toString() = "FinalToString"
}

abstract class AbstractToString {
    abstract override fun toString(): String
}

data object InheritsFinalToString : FinalToString()
data object InheritsAbstractToString : AbstractToString()

data class Cloned(var x: Int) : Cloneable {
    public override fun clone(): Cloned = super.clone() as Cloned
}

class Outer(val s: String) {
    inner class In(val t: String) {
        // A local data class that captures the instances of In and Outer.
        fun local(): String {
            data class L(val x: Int) {
                fun show() = s + t + x
            }
            val l = L(1)
            return l.copy(x = 2).show() + " " + l.copy()
        }
    }
}

annotation class Anno(val v: Int = Anno.X) {
    companion object {
        const val X = 7
        val name = "Anno"
        fun greet() = "hello " + name
    }
}

interface IFoo {
    fun foo(): String
}

abstract class Base(val x: IFoo)

enum class Entry : IFoo {
    FOO {
        inner class Inner : Base(FOO) {
            val fooFoo = Entry.FOO.foo()
            inner class Deeper : Base(FOO)
            val deeper = Deeper()
        }

        inner class Delegating : IFoo by FOO

        val z = Inner()
        val d = Delegating()

        override fun foo() = "foo"
        override fun bar() = z.x.foo() + " " + z.fooFoo + " " + z.deeper.x.foo() + " " + d.foo()
    };

    abstract fun bar(): String
}

fun localData(str: String): String {
    var captured = 1
    data class A(val x: Int) {
        var y = x + captured
        fun foo() = str + x + y
    }
    captured = 10
    val a = A(1)
    val b = a.copy()
    val c = a.copy(x = 2)
    return a.foo() + " " + b.foo() + " " + c.foo() + " " + (a == b) + " " + b
}

fun main() {
    println(localData("s"))
    val d = InheritsFinal("x")
    println(d.toString() + " " + d.hashCode() + " " + d.equals(d) + " " + d.component1() + " " + d.copy("y").field)
    println(InheritsFinalToString.toString() + " " + InheritsAbstractToString)
    val a = Cloned(42)
    val b = a.clone()
    b.x = 1
    println("" + (a == b) + " " + (a === b) + " " + a + " " + b)
    println(Outer("o").In("i").local())
    println(Anno.X.toString() + " " + Anno.name + " " + Anno.greet())
    println(Entry.FOO.bar())
}

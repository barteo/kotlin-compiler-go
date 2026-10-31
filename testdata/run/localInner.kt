// Inner classes of local classes and of object expressions' classes: they
// read what the outer class captures through this$0, as kotlinc's do, in
// their methods, constructors, default arguments and lambdas, and see the
// enclosing class's this. And a lambda in a local class that constructs
// the class captures what the class's later code captures.

open class Base(val tag: String)

class Host(val name: String) {
    fun make(s: String, n: Int): String {
        var count = 0
        class Local(val k: Int) {
            val y = n + k

            inner class Inner(val z: Int = n + y) : Base(s + z) {
                val w = s + count

                fun result(): String {
                    count++
                    return "$s $y $z $w ${this@Host.name} ${this@Local.k} $tag"
                }

                inner class Deep {
                    fun r() = s + n + result() + { count + y }()
                    fun again() = Local(k + 1).Inner(0).result()
                }
            }

            fun inner() = Inner()
        }
        val d = Local(1).inner().Deep()
        return d.r() + " | " + d.again() + " | " + count
    }
}

fun obj(prefix: String): String {
    val o = object {
        val z = 1

        inner class I(val x: Int) {
            fun r() = prefix + z + x
        }

        fun go() = I(2).r()
    }
    return o.go()
}

fun loop(): String {
    var log = ""
    var s: Any? = null
    for (t in arrayOf("1", "2", "3")) {
        class C {
            val y = t

            inner class D {
                fun copyOuter() = C()
            }
        }
        if (s == null) s = C()
        log += (s as C).D().copyOuter().y
    }
    return log
}

fun copier(): String {
    val k = 1
    class B(var x: Int) {
        var y = 0
        fun copier(): () -> B = { B(x) }
        init { y += x + k }
    }
    return "" + B(100).copier()().y
}

fun <T> generic(t: T): T {
    val o = object {
        inner class I {
            fun get(): T = t
        }
    }
    return o.I().get()
}

fun main() {
    println(Host("h").make("a", 2))
    println(obj("p"))
    println(loop())
    println(copier() + " " + generic("g"))
}

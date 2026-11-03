// How the JVM lets code use what Kotlin's visibility does: an interface's
// private members are private methods of the interface, called directly
// in it and through static accessors from its lambdas and objects; and a
// class whose companion object is a subclass of a class of another
// package calls that class's protected members through accessors of the
// companion.

package app

import base.Base

interface Named {
    private val prefix: String get() = "named:"
    private var count: Int
        get() = 1
        set(x) { println("set count $x") }
    private fun tag(x: Int = 1): String = "tag$x"
    private fun String.shout() = this + "!"

    fun describe(): String {
        count = 5
        count += 1
        val lam = { tag(2) + prefix }
        val obj = object {
            fun run() = tag(3) + count + "o".shout()
        }
        return prefix + count + lam() + obj.run() + "x".shout() + tag()
    }

    fun other(n: Named) = n.tag(9)
}

// A private member of an interface isn't inherited: the class's c is A's.
interface A {
    val c: String get() = "A.c"
}

interface B {
    private val c: String get() = "B.c"
    fun bc() = c
}

open class C {
    private val c: String = "C.c"
}

class D : C(), A, B, Named {
    val b = c
}

class UsesCompanion {
    companion object : Base()

    init {
        hello("init")
        val lam = { hello("lambda") }
        lam()
        println(greeting)
        greeting = "hi"
        println(greeting)
    }
}

interface WithCompanion {
    companion object : Base()

    fun test() = hello("interface")
}

class Impl : WithCompanion

fun main() {
    val d = D()
    println(d.describe())
    println(d.other(d))
    println(d.b + " " + d.c + " " + d.bc())
    UsesCompanion()
    Impl().test()
}

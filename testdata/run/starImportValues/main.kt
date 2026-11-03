// Names from star imports of classes, and typealiases, as values: an
// enum's values and valueOf under import E.*, an object under import
// A.B.* where a class of its name isn't a value, an object called
// through its typealias, and a typealias's class literal.
package main

import defs.E.*
import defs.AO
import defs.S
import main.Host.B.*

private class C

class Host {
    fun c(): String { val v = C; return v.name }
    private class B {
        object C { val name = "Host.B.C" }
    }
}

fun main() {
    println(values().size)
    println(valueOf("ANOTHER"))
    println(ENTRY)
    println(Nested.foo())
    println(Host().c())
    println(AO())
    println(S::class.java.simpleName)
}

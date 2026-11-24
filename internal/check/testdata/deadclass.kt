// The warnings kotlinc 2.4.21 reports with -Wextra of the code of the
// classes below that can't run, after a property's initializer that
// always jumps, as TestWarningsDeadClass reads them: each line ends with
// a comment naming the warnings it reports, if any. They follow the
// tests deadNestedDeclaration.kt and deadAnonymousObject.kt of kotlinc's
// diagnostics tests.
@file:Suppress("UNUSED_PARAMETER", "UNUSED_VARIABLE")

fun fail(): Nothing = throw RuntimeException()

class Lambdas {
    val x: String = fail()
    val o = { // UNREACHABLE_CODE
        val a = 0
    } // UNREACHABLE_CODE
    val f = fun() { // UNREACHABLE_CODE
        val a = 0
    } // UNREACHABLE_CODE
    val l: String by lazy { // UNREACHABLE_CODE
        "a"
    } // UNREACHABLE_CODE
    val n = 1 // UNREACHABLE_CODE
}

class Init {
    init {
        val a = 0
    }
    val x: String = fail()
    init {
        val a = 0 // UNREACHABLE_CODE
        class A // UNREACHABLE_CODE
    }
    constructor(a: Int) { // UNREACHABLE_CODE
        val c = 0 // UNREACHABLE_CODE
    }
}

class Object {
    val x: String = fail()
    val o = object { // UNREACHABLE_CODE
        val a = 0
        val b = {
            val c = 0
        } // UNREACHABLE_CODE
        val d: Int by lazy { // UNREACHABLE_CODE
            1
        } // UNREACHABLE_CODE
    }
}

class Untouched {
    val x: String = fail()
    class Nested {
        val a = { 0 }
        init {
            val b = 0
        }
    }
    fun foo() {
        val a = { 0 }
    }
    val g: Int
        get() = 1
}

class Reachable {
    val x: String? = null
    val y = x!!
    val z = { 0 }
    init {
        val a = 0
    }
}

class InitJumps {
    init {
        fail()
    }
    val y = 1 // UNREACHABLE_CODE
}

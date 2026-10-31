import kotlin.reflect.KProperty

class ReadOnly {
    operator fun getValue(thisRef: Any?, p: KProperty<*>): Int = 1
}

interface I {
    val x: Int by lazy { 1 } // ERROR delegated properties are not allowed in interfaces
}

class C {
    val a: Int by 5 // ERROR unresolved reference: getValue
    var b: Int by ReadOnly() // ERROR unresolved reference: setValue
    val c: String by ReadOnly() // ERROR inferred type is Int but String was expected
    val d by lazy { "d" }
}

fun main() {
    var v by lazy { 1 } // ERROR unresolved reference: setValue
    val w by ReadOnly()
    w = 2 // ERROR val cannot be reassigned
    val s: String by ReadOnly() // ERROR inferred type is Int but String was expected
    C().d = "e" // ERROR val cannot be reassigned
}

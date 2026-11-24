package b

import a.*
import a.B as Renamed
import kotlin.collections.List as KotlinList
import java.util.List as JavaList
import kotlin.io.println as say
import java.lang.Thread as JavaThread

fun f(b: B) {} // ERROR unresolved or unsupported type: B
fun g(a: A, r: Renamed) {}
fun h(l: List<Int>) {} // ERROR unresolved or unsupported type: List
fun i() {
    println() // ERROR unresolved reference: println
    say()
}
fun j(t: Thread) {} // ERROR unresolved or unsupported type: Thread

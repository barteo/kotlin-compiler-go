// An import under another name hides the simple name of what it imports
// in the file's package, the star imports and the default imports, but
// not where the same declaration is imported under its own name too, nor
// a declaration of that name in another package.
package a

import a.A as ER
import a.B as BB
import a.B
import a.x as y
import a.foo as bar
import kotlin.collections.map as map1
import kotlin.Array as KotlinArray
import java.util.ArrayList as JavaArrayList
import java.util.HashMap as JavaHashMap

val x: Int = 1
fun foo(): Int = 1
class A
class B

interface I {
    val a: A // ERROR unresolved or unsupported type: A
    val b: ER
}

fun main() {
    A() // ERROR unresolved reference: A
    ER()
    a.A()
    x + 1 // ERROR unresolved reference: x
    y + 1
    foo() + 1 // ERROR unresolved reference: foo
    bar() + 1
    B()
    BB()
    listOf(1).map1 { it.hashCode() }
    listOf(1).map { it.hashCode() } // ERROR unresolved reference: map
    sequenceOf(1).map { it }
    val list: ArrayList<Int> = ArrayList()
    val map = HashMap<Int, Int>()
}

fun g(a1: KotlinArray<Int>, a2: Array<Int>) {} // ERROR unresolved or unsupported type: Array

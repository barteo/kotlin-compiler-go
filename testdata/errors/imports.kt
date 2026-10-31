package p

// Imports of members of classes and objects.

import p.C.inst // ERROR cannot import 'inst'. Functions and properties can only be imported from packages or objects
import p.C.nope // ERROR unresolved import: p.C.nope
import p.O.nope2 // ERROR unresolved import: p.O.nope2
import p.O.x
import p.E.A
import p.O.* // ERROR cannot import on demand from object 'O'

class C { val inst = 1 }
object O { val x = 1 }
enum class E { A, B }

fun f(e: E) = when (e) { A -> 1 } // ERROR 'when' expression must be exhaustive

fun main() {
    x = 2 // ERROR val cannot be reassigned
}

// As in K2, imports of one level that give a name two distinct
// declarations make it ambiguous where it is used.
package c

import a.*
import b.*

fun types(x: X) {} // ERROR overload resolution ambiguity

val literal = X::class.java

val qualified = Some.foo // ERROR overload resolution ambiguity

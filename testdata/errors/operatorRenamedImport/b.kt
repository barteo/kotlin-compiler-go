package b

import a.A
import a.plus as minus

// The import renames the operator: minus is no operator.
fun test(a1: A, a2: A) = a1 - a2 // ERROR 'operator' modifier is required

fun named(a1: A, a2: A) = a1.minus(a2)

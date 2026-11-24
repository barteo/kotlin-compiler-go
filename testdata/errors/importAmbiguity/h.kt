// A name that two explicit imports give is ambiguous as a type, and an
// unresolved call.
package h

import a.X // ERROR conflicting import
import b.X // ERROR conflicting import

fun type(x: X) {} // ERROR overload resolution ambiguity

fun call() = X() // ERROR unresolved reference

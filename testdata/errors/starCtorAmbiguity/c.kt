// As in K2, a class and functions of its name star-imported at one level
// are candidates together: a call that none of them fits best is
// ambiguous, an abstract class's constructor included.
package c

import a.*
import b.*

fun take(arg: Any) {}

fun test() {
    Cls() // ERROR overload resolution ambiguity
    take(Cls()) // ERROR overload resolution ambiguity
    Open() // ERROR overload resolution ambiguity
    Specific("") // the constructor is more specific
    Cls2()
}

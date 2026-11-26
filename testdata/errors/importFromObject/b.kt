// A star import of an object is reported but imports its members all
// the same, as K2 does, and import a.S.A imports the object's class A
// and its function A.
package b

import a.O.* // ERROR cannot import on demand from object 'O'
import a.S.A

fun t() {
    A()
    println(B)
    bar()
    A(3)
}

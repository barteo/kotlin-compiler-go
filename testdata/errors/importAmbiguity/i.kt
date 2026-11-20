// A class deprecated with level HIDDEN, imported explicitly, gives way
// to a star import's class of the name, as K2's.
package i

import a.H
import b.*

fun use(h: H) {
    h.m2()
    h.m1() // ERROR unresolved reference
}

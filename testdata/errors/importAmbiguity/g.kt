// A constructor call of a name that two star imports give, none of
// whose classes a default import has, is unresolved, as K2's.
package g

import a.*
import b.*

fun call() = X() // ERROR unresolved reference

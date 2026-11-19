// A call of an explicitly imported typealias's constructor whose type
// arguments are out of the bounds of its class's goes to the package's
// classifier of that name, as in Kotlin 2.4
// (ProperlyCheckUpperBoundsViolationsWhenCreatingFreshVariables); a type
// still names the import.
package b

import a.Alias
import a.Bound

class Box<B>
typealias Alias<K> = Box<K>

val box: Box<Any> = Alias<Any>()
val base: a.Base<Bound> = Alias<Bound>()
val notBase: a.Base<Bound> = Alias<Any>() // ERROR type mismatch: inferred type is Box<Any>
val notBox: Box<Bound> = Alias<Bound>() // ERROR type mismatch: inferred type is Base<Bound>
val type: Alias<Any>? = null // ERROR not within its bounds

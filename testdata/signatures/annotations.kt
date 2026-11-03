// Annotation classes: a generic one's type parameters, and the generic
// signatures of the methods of KClass parameters, as Class's.

package a

import kotlin.reflect.KClass

annotation class Typed<T : Any>(val k: KClass<T>, val n: Int = 0)

annotation class Classes(val one: KClass<*>, val many: Array<KClass<out Number>>, val n: UInt = 1u)

annotation class Both<A, B : CharSequence>(val name: String)

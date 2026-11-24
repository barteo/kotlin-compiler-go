value class NoAnnotation(val x: Int) // ERROR value classes without '@JvmInline' annotation are not yet supported

@JvmInline
value class Two(val x: Int, val y: Int) // ERROR value class must have exactly one primary constructor parameter

@JvmInline
value class NotVal(x: Int) // ERROR value class primary constructor must only have final read-only ('val') property parameters

@JvmInline
value class IsVar(var x: Int) // ERROR value class primary constructor must only have final read-only ('val') property parameters

@JvmInline
value class Field(val x: Int) {
    val y: Int = 1 // ERROR value class cannot have properties with backing fields
}

open class Base

@JvmInline
value class Extends(val x: Int) : Base() // ERROR value class cannot extend classes

@JvmInline
open value class Open(val x: Int) // ERROR value class can be only final

@JvmInline
value class OfUnit(val x: Unit) // ERROR value class cannot have value parameter of type 'kotlin/Unit'

@JvmInline
value class Recursive(val x: Recursive) // ERROR value class cannot be recursive

@JvmInline
value class Mutual1(val x: Mutual2) // ERROR value class cannot be recursive

@JvmInline
value class Mutual2(val x: Mutual1) // ERROR value class cannot be recursive

@JvmInline
value class Ok(val x: Int) {
    fun withDefault(y: Int = 1): Int = x + y
}

interface Named {
    fun name(): String
}

@JvmInline
value class ByOther(val x: Int) : Named by object : Named { // ERROR value class cannot implement an interface by delegation if expression is not a parameter
    override fun name() = "other"
}

fun identity(a: Ok, b: Ok) = a === b // ERROR identity equality for arguments of types 'Ok' and 'Ok' is prohibited

@JvmInline
value class Generic<T>(val x: T)

fun same(a: Generic<Int>, b: Generic<Int>) = a === b // ERROR identity equality for arguments of types 'Generic<Int>' and 'Generic<Int>' is prohibited

@JvmInline // ERROR '@JvmInline' annotation is applicable only to value classes
inline class Deprecated(val x: Int)

inline class DeprecatedTwo(val x: Int, val y: Int) // ERROR value class must have exactly one primary constructor parameter

fun values(vararg vs: Extends) {} // ERROR prohibited vararg parameter type 'Extends'
fun nullableValues(vararg vs: Extends?) {} // ERROR prohibited vararg parameter type 'Extends?'

// As kotlinc 2.4, which allows none of these without the language feature
// CustomEqualsInValueClasses, and reports a deprecated inline class too.
class Outer {
    @JvmInline
    inner value class InnerValue(val x: Int) // ERROR value class cannot be local or inner
    inline inner class InnerInline(val x: Int) // ERROR value class cannot be local or inner
}

fun localValue() {
    @JvmInline value class Local(val x: Int) // ERROR value class cannot be local or inner
}

@JvmInline
value class CloneableValue(val x: Int) : Cloneable // ERROR value class cannot be 'Cloneable'

inline class OpenParam(open val x: Int) // ERROR final read-only ('val') property parameters

@JvmInline
value class OwnEquals(val x: Int) {
    override fun equals(other: Any?) = true // ERROR member name 'equals' is reserved for future releases
    override fun hashCode() = 1 // ERROR member name 'hashCode' is reserved for future releases
    fun box() = 1 // ERROR member name 'box' is reserved for future releases
    fun toBox() = 1
}

interface WithBox {
    fun box(): Int = 1
    fun <T> equals(other: T, x: Int): Boolean = true
}

interface AbstractUnbox {
    fun unbox(): Int
}

@JvmInline
value class InheritsBox(val x: Int) : WithBox // ERROR member name 'box' is reserved for future releases but is implemented in supertype 'WithBox'

@JvmInline
value class ImplementsUnbox(val x: Int) : AbstractUnbox {
    override fun unbox() = x // ERROR member name 'unbox' is reserved for future releases
}

@Suppress("RESERVED_MEMBER_INSIDE_VALUE_CLASS")
@JvmInline
value class SuppressedEquals(val x: Int) {
    override fun equals(other: Any?) = other is SuppressedEquals
}

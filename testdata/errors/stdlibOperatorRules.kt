// What kotlinc requires of operator functions' parameters and results.
import kotlin.reflect.KProperty

class Example {
    operator fun minus(vararg o: Example): Example = this // ERROR should not have varargs or parameters with default values
    operator fun plus(o: Example): Example = this
    operator fun minusAssign(n: Int): String = "" // ERROR must return Unit
    operator fun plusAssign(n: Int) {}
    operator fun dec(): Example? = null // ERROR receiver must be a supertype of the return type
    operator fun inc(): Example = this
    operator fun hasNext(): Int = 0 // ERROR must return Boolean
    operator fun equals(a: String): Boolean = false // ERROR must override ''equals()'' in Any
    operator fun getValue(thisRef: Any?, prop: String): String = "" // ERROR second parameter must be of type KProperty<*> or its supertype
    operator fun getValue(thisRef: Any?, prop: KProperty<*>): String = ""
    operator fun setValue(thisRef: Any?, prop: KProperty<*>, f: Float = 0.0f) {} // ERROR should not have varargs or parameters with default values
    operator fun mod(x: Int) {} // ERROR illegal function name
    operator fun rem(x: Int) {}
}

operator fun Example.equals(a: Any?): Boolean = false // ERROR must be a member function
operator fun Example.times(o: Example) = 1
operator fun Example.timesAssign(o: Example) = 1 // ERROR must return Unit

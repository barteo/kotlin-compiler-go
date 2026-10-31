interface I {
    fun f(): Int
    val p: String
}

open class A {
    open fun g(): Int = 1
    fun h(): Int = 2
    open var v: Int = 0
    final override fun toString(): String = "A"
}

class B : A() {
    fun g(): Int = 3 // ERROR 'g' hides member of supertype 'A' and needs 'override' modifier
    override fun h(): Int = 4 // ERROR 'h' in 'A' is final and cannot be overridden
    override fun k(): Int = 5 // ERROR 'k' overrides nothing
    override val v: Int = 1 // ERROR val-property cannot override var-property 'v'
    override fun toString(): String = "B" // ERROR 'toString' in 'A' is final and cannot be overridden
}

class C : I { // ERROR class 'C' is not abstract and does not implement abstract member 'f'
}

class D : I {
    override fun f(): String = "" // ERROR return type of 'f' is not a subtype of the return type of the overridden member
    override val p: String = ""
}

class Final
class E : Final() // ERROR this type is final, so it cannot be inherited from
class F : A(), Final() // ERROR only one class may appear in a supertype list
class G : A // ERROR this type has a constructor, and thus must be initialized here
class H : I() // ERROR this type has no constructor, and thus must not be called
interface J : A() // ERROR an interface cannot inherit from a class

open class Cycle1 : Cycle2() // ERROR there's a cycle in the inheritance hierarchy for this type
open class Cycle2 : Cycle1() // ERROR there's a cycle in the inheritance hierarchy for this type

abstract class K {
    abstract fun m(): Int
    fun n(): Int = 1
}

class L : K() {
    override fun m(): Int = super.m() // ERROR abstract member cannot be accessed directly
}

class M {
    abstract fun m(): Int // ERROR abstract function 'm' in non-abstract class 'M'
    fun noBody(): Int // ERROR function 'noBody' must have a body
}

fun make() {
    val i = I() // ERROR cannot create an instance of an interface: I
    val k = K() // ERROR cannot create an instance of an abstract class: K
    val s = super.toString() // ERROR 'super' is not
}

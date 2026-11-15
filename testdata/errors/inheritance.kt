// The checks of inherited members: visibility, default values and
// interfaces that implement Any's methods.

interface Shape {
    val area: Int
    var name: String
    fun draw()
}

class Square : Shape {
    internal override val area: Int = 1 // ERROR cannot weaken access privilege 'public'
    override var name: String = ""
        private set // ERROR cannot weaken access privilege 'public'
    protected override fun draw() {} // ERROR cannot weaken access privilege 'public'
}

open class Base {
    protected open fun f() {}
    internal open fun g() {}
}

class Derived : Base() {
    internal override fun f() {} // ERROR cannot change access privilege 'protected'
    protected override fun g() {} // ERROR cannot change access privilege 'internal'
}

// An override without a visibility has the overridden member's.
open class Mid : Base() {
    override fun g() {}
}

class Leaf : Mid() {
    internal override fun g() {}
}

interface X {
    fun foo(a: Int = 1)
}

interface Y {
    fun foo(a: Int = 2)
}

class XY : X, Y {
    override fun foo(a: Int) {} // ERROR more than one overridden descriptor declares a default value for 'a'
}

interface WithAny {
    override fun toString(): String = "" // ERROR an interface may not implement a method of 'Any'
    override fun equals(other: Any?): Boolean // abstract: allowed
    fun hashCode(x: Int): Int = x // not Any's
}

// Members inherited from more than one supertype.
interface HasFoo {
    fun foo(): Int = 1
}

interface AlsoFoo {
    fun foo(): Int = 2
}

interface AbstractFoo {
    fun foo(): Int
}

open class OpenFoo {
    open fun foo(): Int = 3
}

class TwoDefaults : HasFoo, AlsoFoo // ERROR must override 'foo' because it inherits multiple interface methods

class DefaultAndAbstract : HasFoo, AbstractFoo // ERROR must override 'foo' because it inherits multiple interface methods

class ClassAndDefault : OpenFoo(), HasFoo // ERROR must override 'foo' because it inherits many implementations

class ClassAndAbstract : OpenFoo(), AbstractFoo

open class ResolvedFoo : HasFoo, AlsoFoo { // ERROR must override 'foo'
}

class FromResolved : ResolvedFoo() // checked in ResolvedFoo

interface StrF {
    fun f(): String
}

open class SeqF {
    open fun f(): CharSequence = ""
}

class SeqStr : SeqF(), StrF // ERROR return types of inherited members 'f' are incompatible

interface IntF {
    fun f(): Int
}

abstract class StrInt : StrF, IntF // ERROR return types of inherited members 'f' are incompatible

interface StrP {
    val p: String
}

interface IntP {
    val p: Int
}

abstract class StrIntP : StrP, IntP // ERROR the types of the inherited property 'p' don't match

open class InternalP {
    internal val q: Int get() = 1
}

interface PublicQ {
    val q: Int
}

abstract class Weakened : InternalP(), PublicQ // ERROR cannot weaken access privilege 'internal' for 'q'

interface Defaults1 {
    fun g(x: Int = 1)
}

open class Defaults2 {
    open fun g(x: Int = 2) {}
}

class BothDefaults : Defaults2(), Defaults1 // ERROR more than one inherited member declares a default value

abstract class AbstractInClass {
    abstract fun h(): String
}

interface DefaultH {
    fun h(): String = ""
}

abstract class AbstractH : AbstractInClass(), DefaultH

class CallsAbstract : AbstractH() {
    override fun h(): String = super.h() // ERROR abstract member cannot be accessed directly
}

interface First {
    operator fun component1(): Int = 1
}

data class Pair1(val a: Int) : First // ERROR overrides an implementation of a componentN function

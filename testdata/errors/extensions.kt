// Extension functions, and the operator and infix modifiers.

fun String.shout() = this + "!"
fun <T> T.self() = this
val String.twice: String get() = this + this
val String.inited: Int = 1 // ERROR extension property cannot be initialized because it has no backing field
val Int.noGetter: Int // ERROR extension property must have accessors or be abstract
var Int.mutable: Int get() = this // ERROR extension property must have accessors or be abstract
var Int.setterOnly: Int set(v) {} // ERROR extension property must have accessors or be abstract
fun String.bad() = nope // ERROR unresolved reference: nope
fun String.badLabel() = this@other // ERROR unresolved label: other
fun Int.typed(): String = this // ERROR type mismatch

open class A {
    fun Int.double() = this * 2
    val Int.triple: Int get() = this * 3
    open val Int.quadruple: Int get() = this * 4
    open fun Int.opened() = 1
    infix fun two(a: Int, b: Int) = a + b // ERROR 'infix' modifier is inapplicable on this function: must have a single value parameter
    infix fun many(vararg a: Int) = 0 // ERROR must have a single value parameter
    infix fun dflt(a: Int = 1) = a
    infix fun ok(a: Int) = a
    operator val p = 1 // ERROR modifier 'operator' is not applicable to 'property'
}

infix fun top(a: Int) = a // ERROR 'infix' modifier is inapplicable on this function: must be a member or an extension function
infix class C // ERROR modifier 'infix' is not applicable to 'class'
fun Int.plain(x: Int) = this + x

class B {
    fun get(i: Int) = i
    fun plus(b: B) = b
}

fun main() {
    println(A() ok 1)
    println(B()[0]) // ERROR 'operator' modifier is required on 'get' in 'B'
    println(B() plus B()) // ERROR 'infix' modifier is required on 'plus'
    println(5.shout()) // ERROR unresolved reference: shout
    println(shout()) // ERROR unresolved reference: shout
    val n: String? = null
    println(n.shout()) // ERROR only safe (?.) or non-null asserted (!!.) calls are allowed on a nullable receiver of type String?
    println(1 plain 2) // ERROR 'infix' modifier is required on 'plain'
    println("a".self().length)
    println("ab".twice.length)
    println(5.double()) // ERROR unresolved reference: double
    val m: String? = null
    println(m.twice) // ERROR only safe (?.) or non-null asserted (!!.) calls are allowed on a nullable receiver of type String?
    println(1.nope) // ERROR unresolved reference: nope
}

interface Builds {
    fun Int.built(): String
}

class Forgets : Builds // ERROR class 'Forgets' is not abstract and does not implement abstract member 'built'

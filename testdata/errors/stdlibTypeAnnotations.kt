// The annotations of types and of type parameters.

import kotlin.reflect.KClass

annotation class Decl
@Target(AnnotationTarget.TYPE)
annotation class TypeUse(val n: Int = 0)
@Target(AnnotationTarget.TYPE_PARAMETER)
annotation class TypeParam

fun f(x: @Decl Int) {} // ERROR this annotation is not applicable to target 'type usage'
fun g(): List<@Decl String> = listOf() // ERROR this annotation is not applicable to target 'type usage'
fun h(x: @TypeUse(1) Int?, y: @TypeUse("") Int) {} // ERROR type mismatch
fun i(x: @TypeUse @TypeUse Int) {} // ERROR this annotation is not repeatable
fun j(x: @Unknown Int) {} // ERROR unresolved reference: Unknown

class C<@Decl T> // ERROR this annotation is not applicable to target 'type parameter'
fun <@TypeParam T, @TypeUse U> k() {} // ERROR this annotation is not applicable to target 'type parameter'
fun <@TypeParam @TypeParam T> l() {} // ERROR this annotation is not repeatable

fun body(a: Any) {
    if (a is @Decl String) return // ERROR this annotation is not applicable to target 'type usage'
    val x: @TypeUse(2) Any = a
}

// The arguments are those of the class the type is written in.
@Target(AnnotationTarget.TYPE)
annotation class K(val k: KClass<*>)
class Outer {
    fun f(): @K(Nested::class) Any = 0
    private inner class Inner
    fun g(): @K(Inner::class) Any = 0
    class Nested
}

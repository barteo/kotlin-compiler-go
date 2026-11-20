// The warnings of type parameters' bounds declared both with them and in
// a where clause below, as TestWarningsMisplacedConstraints reads them:
// each line ends with a comment naming the warnings it reports, if any.
// They follow the tests misplacedConstraints.kt and kt46483.kt of kotlinc
// 2.4.21's diagnostics tests.
@file:Suppress("UNUSED_PARAMETER")

@Target(AnnotationTarget.TYPE_PARAMETER)
annotation class Ann

interface I<T>

class Foo<T : Cloneable> where T : Comparable<T> { // MISPLACED_TYPE_PARAMETER_CONSTRAINTS
    fun <U : Cloneable> foo(u: U): U where U : Comparable<U> = u // MISPLACED_TYPE_PARAMETER_CONSTRAINTS
}

val <U : Cloneable> U.prop: U? where U : Comparable<U> get() = null // MISPLACED_TYPE_PARAMETER_CONSTRAINTS

fun <@Ann T : Any> annotated() where T : I<String> {} // MISPLACED_TYPE_PARAMETER_CONSTRAINTS

interface Correct<T> where T : Cloneable, T : Comparable<T>

fun <T : Any, U> other(t: T, u: U) where U : Comparable<U> {}

fun <T : Any> plain(t: T) {}

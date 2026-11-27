// A type parameter can't be named _ but in backticks, and a type
// argument naming such a parameter must write it in backticks too.

class Foo<K>
class Bar<_> // ERROR names _, __, ___, ... are reserved in Kotlin
class Baz<`_`>

typealias A<_> = Foo<_> // ERROR names _, __, ___, ... are reserved in Kotlin
typealias B<`_`> = Foo<Foo<_>> // ERROR can be used only in back-ticks
typealias C<`_`> = Foo<Foo<`_`>>

fun <`_`> bar(): Foo<_> = TODO() // ERROR can be used only in back-ticks
fun <`_`> ok(): Foo<`_`> = TODO()

fun <T> id(x: T) = x
fun inferred(): Int = id<_>(1)

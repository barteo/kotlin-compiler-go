package interop

// Kotlin declarations the Java classes of the package extend.
interface Sized {
    val size2: Int
}

open class Base {
    open val label: String get() = "base"
}

open class Derived : Base() {
    override val label: String get() = "derived"
}

open class Getters {
    open fun getTitle(): String = "title"
}

abstract class Counter {
    open val count: Int get() = 7
}

class FromJava : JavaBase()

class Tally : JavaTally()

// A Kotlin property implements the getter of a Java interface that
// extends a Kotlin one.
interface Described {
    val text: String
}

class Description : JavaDescribed {
    override val text: String get() = "described"
}

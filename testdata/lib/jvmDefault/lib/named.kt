// kotlinc: -jvm-default=enable
package lib

// Interfaces compiled with -jvm-default=enable: their bodies are default
// methods, and also static methods of DefaultImpls classes, which
// sub-interfaces' have too, for code compiled to call them there; and
// classes that inherit default methods get bridges to them.
interface Named {
    val name: String
    val greeting: String get() = "hello " + name
    var volume: Int
        get() = 1
        set(value) {}
    fun describe(prefix: String = ">"): String = prefix + " " + name
    fun <T> echo(t: T): T = t
    fun shout(): String
}

interface Loud : Named {
    override fun shout() = greeting.uppercase()
    fun twice() = shout() + shout()
}

interface Silent : Loud {
    override fun twice(): String
}

// Without a DefaultImpls class.
@JvmDefaultWithoutCompatibility
interface Quiet {
    fun whisper() = "psst"
}

// Only name$default methods in its DefaultImpls class.
interface Registry {
    fun register(name: String, eager: Boolean = false): String
}

open class Base(override val name: String) : Loud, Quiet {
    override fun describe(prefix: String) = "base " + super.describe(prefix)
}

class Leaf : Base("leaf")

package d

annotation class Tag

@Target(AnnotationTarget.VALUE_PARAMETER)
annotation class ParamTag

@JvmInline
value class Id(val value: String)

@JvmInline
value class Named(val v: Int) {
    override fun toString(): String = "Named"
}

data class Point(@Tag val x: Int, @ParamTag val y: Int, @property:Tag val z: Int, @get:Tag val w: Int = 0)

data class Labeled(val label: String) {
    override fun hashCode(): Int = 1
}

data object None

class Mixed {
    val b: Int = 1
    fun a() = 1
    val a: Int = 2
    fun b(x: Int) = x
    fun c() = 3
}

interface Source<T> {
    val name: String
    var count: Int
    fun read(x: T): T
    fun size(): Int = 1
    override fun toString(): String
}

open class Base {
    open fun base() = 1
}

class Wrapped(s: Source<String>) : Base(), Source<String> by s {
    override fun size(): Int = 2
}

class Plain(val s: Source<Int>) : Source<Int> by s

@Suppress("UNUSED")
class Suppressed

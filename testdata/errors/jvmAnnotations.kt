// The JVM annotations kc doesn't model are reported, rather than giving
// Java callers other members than kotlinc would.

interface Base { fun id(): Int }

object Registry : Base {
    @JvmStatic fun register() = 1
    @JvmField val names = "a"
    @JvmStatic val count = 2
    @JvmStatic override fun id() = 3 // ERROR override member cannot be '@JvmStatic' in an object
    @JvmStatic fun Int.twice() = this * 2 // ERROR @JvmStatic is not supported on extension functions of objects
    @JvmStatic val Int.half: Int get() = this / 2 // ERROR @JvmStatic is not supported on extension properties
    @JvmStatic val lazyOne by lazy { 1 } // ERROR @JvmStatic is not supported on delegated properties
}

class Holder {
    companion object {
        @JvmStatic fun make() = Holder()
        @JvmStatic val shared = 1
        @JvmField val field = 2
    }
    @JvmStatic fun notInObject() = 1 // ERROR only members in named objects and companion objects can be annotated with '@JvmStatic'
}

interface WithCompanion {
    companion object {
        @JvmField var mutable = 1 // ERROR JvmField could be applied only if all interface companion properties are 'public final val' with '@JvmField' annotation
    }
}

fun main() {
    val o = object {
        @JvmStatic fun f() = 1 // ERROR only members in named objects and companion objects can be annotated with '@JvmStatic'
    }
    println(o.f())
}

open class Base {
    open inline fun opened() {} // ERROR 'inline' modifier is not allowed on virtual members
    inline fun closed() {}
    private inline fun hidden() {}
    open inline val prop: Int get() = 1 // ERROR 'inline' modifier is not allowed on virtual members
    open val getter: Int inline get() = 1 // ERROR 'inline' modifier is not allowed on virtual members
}

interface Iface {
    inline fun member() {} // ERROR 'inline' modifier is not allowed on virtual members
    private inline fun helper() {}
}

open class Derived : Base() {
    override fun toString() = ""
    final override inline fun opened() {}
}

class Final : Base() {
    override inline fun opened() {}
}

class Fields {
    inline val withField: Int = 1 // ERROR inline property cannot have a backing field
    inline val computed: Int get() = 1
}

private class Hidden {
    fun f() = 1
}

private inline fun usesHidden() = Hidden().f()

inline fun exposes() = usesHidden() // ERROR private type 'Hidden' is used in non-private inline function 'exposes' through 'usesHidden'
private inline fun alsoPrivate() = usesHidden()
fun notInline() = usesHidden()

fun main() {}

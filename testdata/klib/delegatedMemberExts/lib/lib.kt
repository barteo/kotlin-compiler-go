package lib

// An interface with member extensions, as Compose's ColumnScope, whose
// methods take the extension receiver first.
interface Scope {
    fun String.weight(w: Float): String
    fun plain(): Int
    fun String.align(): String = "aligned $this"
}

class Impl : Scope {
    override fun String.weight(w: Float) = "$this:$w"
    override fun plain() = 1
}

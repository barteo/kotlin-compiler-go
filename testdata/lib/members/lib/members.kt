package p

// Declarations a program reaches from the class path: extension
// properties, one a var, a top-level property, a companion's invoke
// operator and member extension, and an extension that only fits one
// receiver.

class Tag(var text: String)

var Tag.label: String
    get() = "[$text]"
    set(v) {
        text = v
    }

val Tag.size: Int get() = text.length

val VERSION: Int get() = 3

class Name private constructor(val s: String) {
    companion object {
        operator fun invoke(s: String) = Name(s.uppercase())
        fun Name.shout() = "$s!"
    }
}

fun Tag.render(n: Int) = "lib $text $n"

// componentN have their properties' visibility.
data class Point(private val x: Int, val y: Int) {
    fun sum() = component1() + component2()
}

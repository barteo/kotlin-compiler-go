package objs

// Inline functions that create objects: a program compiled against the
// library copies their bodies, which create the objects' classes, from
// the library's bytecode. kotlinc copies those classes too.

interface Wrapper {
    fun run(): String
}

inline fun wrap(crossinline block: () -> String): Wrapper = object : Wrapper {
    override fun run() = "[" + block() + "]"
}

inline fun counter(start: Int): Wrapper {
    var n = start
    return object : Wrapper {
        override fun run(): String {
            n++
            return "n=" + n
        }
    }
}

class Host(val name: String) {
    inline fun greeter(crossinline suffix: () -> String): Wrapper = object : Wrapper {
        override fun run() = "hi " + name + suffix()
    }
}

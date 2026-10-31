// An inner class's own properties come before its outer class's of the
// same name, also in a lambda with a receiver that doesn't have them, as
// kotlinx.coroutines' select clauses use them.

class Select {
    private var index = -1
    private var handle: Any? = null

    inner class Clause {
        var handle: Any? = null
        var index = -1

        fun register() {
            handle = this@Select.handle
            index = this@Select.index
            this@Select.handle = null
            this@Select.index = -1
        }

        fun dispose(): String = with(handle) {
            if (this is String) "$this $index" else "none $index"
        }

        fun bump() = buildString { index++; append(index) }
    }

    fun select(): String {
        index = 5
        handle = "segment"
        val c = Clause()
        c.register()
        return c.dispose() + " " + c.bump() + " outer $index $handle"
    }
}

fun main() {
    println(Select().select())
}

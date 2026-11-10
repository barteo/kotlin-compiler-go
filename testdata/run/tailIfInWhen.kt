// An if without else that ends a branch of the when a lambda ends with is
// a statement, of type Unit, though the when is the lambda's value.

fun <T> tx(block: () -> T): T = block()

fun early(a: String?, b: Boolean) {
    tx {
        val row = a ?: return@tx
        when {
            row.isEmpty() -> {
                if (b) println("empty")
            }
            else -> println(row)
        }
    }
}

fun subject(a: String, b: Boolean) {
    tx {
        when (a) {
            "" -> {
                if (b) println("empty")
            }
            else -> println(a)
        }
    }
}

fun value(c: Boolean, b: Boolean): Any = tx {
    when {
        c -> {
            if (b) 1
        }
        else -> 2
    }
}

fun nested(c: Boolean, b: Boolean) {
    tx {
        if (c) {
            when {
                b -> {
                    if (b) println("nested")
                }
                else -> {}
            }
        } else {
            println("not c")
        }
    }
}

fun main() {
    early("x", true)
    early("", true)
    early(null, true)
    subject("", true)
    subject("y", false)
    println(value(true, true) is Int)
    println(value(false, true) is Int)
    nested(true, true)
    nested(false, true)
}

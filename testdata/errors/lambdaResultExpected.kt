// A lambda's result that contradicts the type expected of the call, an
// upper bound of its variable, is reported where it is, not at the call.
// An anonymous function with a block body is reported at both.
fun <T> runLike(block: () -> T): T = block()

fun test() {
    var str: String = runLike {
        { } // ERROR type mismatch
    }
    str = runLike {
        { it: String -> it } // ERROR type mismatch
    }
    str = runLike {
        fun() = "" // ERROR type mismatch
    }
    str = runLike { "" }
    str = runLike { // ERROR type mismatch
        fun(): String { // ERROR type mismatch
            return ""
        }
    }
}

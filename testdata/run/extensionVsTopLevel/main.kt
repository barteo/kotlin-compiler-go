package app

import lib.Logging.Priority.INFO
import lib.log

// An extension of an implicit receiver whose parameters don't take the
// arguments gives way to a top-level function that does, also when an
// argument is a property and the call has a lambda: log(TAG, INFO) { }
// calls log(tag: String, ...), not this.log(priority, ...).

private val TAG = "T"
private val P = INFO

class C {
    fun f() {
        log(TAG, INFO) { "a" }
        log(TAG) { "b" }
        log("lit", INFO) { "c" }
        log(INFO) { "d" }
        log { "e" }
        log(P) { "f" }
    }
}

fun main() = C().f()

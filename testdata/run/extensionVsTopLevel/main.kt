package app

import lib.Logging.Priority.INFO
import lib.log

// An extension of an implicit receiver whose parameters don't take the
// arguments gives way to a top-level function that does, also when an
// argument is a property and the call has a lambda: log(TAG, INFO) { }
// calls log(tag: String, ...), not this.log(priority, ...). So does a
// lambda's receiver, as Application's in ktor's embeddedServer { }.

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

class Server

fun <T> T.configure(block: T.() -> Unit) = block()

fun g() = Server().configure {
    log(TAG, INFO) { "g" }
    log(TAG) { "h" }
    log(INFO) { "i" }
}

fun main() {
    C().f()
    g()
}

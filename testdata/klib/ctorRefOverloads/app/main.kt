// A callable reference to a constructor of a library class, Kotlin or
// Java, chooses among overloads by the function types it fits, as one to
// a class of the program does. One to a constructor with parameters with
// default values fits a function type that leaves them out, only if none
// fits it as it is: ::LibOptionalHandler is a (Services) -> H, not a
// (Services, K) -> H.

import h.HandlerBuilder
import h.Handler
import h.JavaHandler
import h.LibHandler
import h.LibOptionalHandler
import h.Services

class AppHandler(services: Services, private val tag: String = "app") : Handler(services) {
    override val name get() = tag
}

class KindHandler(services: Services, kind: String) : Handler(services) {
    override val name = "kind:$kind"
}

fun main() {
    val builder = HandlerBuilder()
    builder.useHandlers(::LibHandler)
    builder.useHandlers(::LibOptionalHandler)
    builder.useHandlers(::JavaHandler)
    builder.useHandlers(::AppHandler)
    builder.useHandlers(::KindHandler)
    builder.useHandlers(::LibHandler, ::JavaHandler)
    println(builder.created)
}

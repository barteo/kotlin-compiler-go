package h

class Services

abstract class Handler(val services: Services) {
    abstract val name: String
}

class LibHandler(services: Services) : Handler(services) {
    override val name = "lib"
}

class LibOptionalHandler(services: Services, strict: Boolean = false) : Handler(services) {
    override val name = if (strict) "strict" else "optional"
}

abstract class Builder<K, H : Handler>(private val kind: K) {
    val created = mutableListOf<String>()

    fun useHandlers(vararg constructors: (Services) -> H) {
        constructors.forEach { created += "one:" + it(Services()).name }
    }

    fun useHandlers(vararg constructors: (Services, K) -> H) {
        constructors.forEach { created += "two:" + it(Services(), kind).name }
    }
}

class HandlerBuilder : Builder<String, Handler>("kind")

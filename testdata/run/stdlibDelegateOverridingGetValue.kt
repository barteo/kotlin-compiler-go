// A delegate whose getValue overrides ReadOnlyProperty's, with a thisRef
// of a type that uses the delegate's type parameters, infers the type
// arguments its function leaves open from the property's type, as one
// with an operator getValue of its own does.

import kotlin.properties.ReadOnlyProperty
import kotlin.reflect.KProperty

interface Service

abstract class Owner<V : Any> {
    val services = mutableMapOf<String, V>()
}

class Services : Owner<Service>() {
    companion object {
        inline fun <reified T : Service> accessor(): Accessor<Service, T> = Accessor(T::class.java.simpleName)
    }
}

class Accessor<V : Any, T : V>(private val key: String) : ReadOnlyProperty<Owner<V>, T> {
    @Suppress("UNCHECKED_CAST")
    override fun getValue(thisRef: Owner<V>, property: KProperty<*>): T = thisRef.services[key] as T
}

class Plain<V : Any, T : V>(private val key: String) {
    @Suppress("UNCHECKED_CAST")
    operator fun getValue(thisRef: Owner<V>, property: KProperty<*>): T = thisRef.services[key] as T
}

fun <T : Service> plain(key: String): Plain<Service, T> = Plain(key)

class Errors : Service {
    override fun toString() = "errors"
}

class Warnings : Service {
    override fun toString() = "warnings"
}

val Services.errors: Errors by Services.accessor()
val Services.warnings: Warnings by plain("Warnings")

fun main() {
    val services = Services()
    services.services["Errors"] = Errors()
    services.services["Warnings"] = Warnings()
    println("${services.errors} ${services.warnings}")
    val local: Errors by lazy { services.errors }
    println(local)
}

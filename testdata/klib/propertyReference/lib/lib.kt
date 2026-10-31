package lib

import kotlin.reflect.KFunction
import kotlin.reflect.KMutableProperty1
import kotlin.reflect.KProperty1

class Price(val currency: String, var cents: Int)

interface Generator<T> {
    val seed: Int
}

// As kotlinfixture's ConfigurationBuilder: property is overloaded, and
// the one for a KProperty1 is inline, with T reified.
class Builder {
    val rules = mutableListOf<String>()

    inline fun <reified T, G> property(propertyName: String, noinline generator: Generator<G>.() -> G) {
        add(T::class.java.simpleName + "." + propertyName, generator)
    }

    inline fun <reified T, G> property(property: KProperty1<T, G>, noinline generator: Generator<G>.() -> G) {
        val kind = if (property is KMutableProperty1<*, *>) "var" else "val"
        add("$kind " + T::class.java.simpleName + "." + property.name, generator)
    }

    fun <G> property(function: KFunction<Unit>, generator: Generator<G>.() -> G) {
        add(function.name, generator)
    }

    fun <G> add(name: String, generator: Generator<G>.() -> G) {
        val g = object : Generator<G> {
            override val seed = rules.size
        }
        rules += name + "=" + g.generator()
    }
}

fun build(configuration: Builder.() -> Unit): List<String> = Builder().apply(configuration).rules

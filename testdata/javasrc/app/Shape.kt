package app

// Kotlin classes that the module's Java classes extend and name.
open class Shape(val name: String) {
    open fun area(): Double = 0.0

    override fun toString() = "$name(${area()})"
}

interface Named {
    fun label(): String
}

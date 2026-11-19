// Lateinit properties of a multi-file class: each part holds its field,
// public as the setter is, and the accessors, which check it; the
// facade's accessors call the part's.
@file:JvmMultifileClass
@file:JvmName("Config")

package cfg

lateinit var name: String

lateinit var owner: String
    private set

internal lateinit var secret: String

fun claim(who: String) {
    // From a lambda and an object expression, which use the part's
    // private setter.
    val set = { owner = who }
    set()
    object : Runnable {
        override fun run() {
            owner += "!"
            println("named: " + ::name.isInitialized)
        }
    }.run()
}

fun named() = ::name.isInitialized

fun owned() = ::owner.isInitialized

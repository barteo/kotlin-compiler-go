@file:JvmMultifileClass
@file:JvmName("Config")

package cfg

lateinit var other: List<Int>

fun describe() = "other=" + (if (::other.isInitialized) other.toString() else "unset") + " secret=" + secret

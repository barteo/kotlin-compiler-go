// @JvmRecord needs a JVM target of 16 or later: kc's default is 1.8, as
// kotlinc's. The error is at the start of the declaration.

@JvmRecord // ERROR using @JvmRecord is only allowed with -jvm-target 16 or later
data class R(val x: Int)

@Suppress("unused") @JvmRecord // ERROR using @JvmRecord is only allowed with -jvm-target 16 or later
class C(val x: Int)

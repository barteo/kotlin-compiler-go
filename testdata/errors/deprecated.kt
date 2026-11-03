@Deprecated("gone", level = DeprecationLevel.ERROR)
fun old() = 1

@Deprecated("going")
fun older() = 2

@Deprecated("hidden", level = DeprecationLevel.HIDDEN)
class Hidden

@Deprecated("gone", level = DeprecationLevel.ERROR)
class Gone

@Deprecated("going", level = DeprecationLevel.WARNING)
class Going

typealias G = Gone // ERROR is deprecated
class Box<T>
typealias L = Box<Gone> // ERROR is deprecated

fun use1() = old() // ERROR is deprecated
fun use2() = older()
fun use3() = Gone() // ERROR is deprecated
fun use4(g: Going) = g
fun use5(l: L) = l // ERROR the expansion of typealias

@Deprecated("whole", level = DeprecationLevel.ERROR)
fun inside() = old()

@Suppress("DEPRECATION_ERROR")
fun suppressed() = old()

class P {
    @Deprecated("gone", level = DeprecationLevel.ERROR)
    val x = 1
}

fun use6(p: P) = p.x // ERROR is deprecated

@SinceKotlin("1.x") // ERROR invalid Kotlin version
fun since() = 1

@DeprecatedSinceKotlin(warningSince = "1.0") // ERROR only allowed in the kotlin package
@Deprecated("x")
fun outside() = 1

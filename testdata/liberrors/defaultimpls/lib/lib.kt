// kotlinc: -jvm-default=disable
package lib

@RequiresOptIn
annotation class Obs

// Without JVM default methods, the properties' $annotations methods are
// in Ctx$DefaultImpls.
interface Ctx {
    @Obs
    val o: Any
    @Deprecated("", level = DeprecationLevel.ERROR)
    val d: Any
    val plain: Any get() = 1
}

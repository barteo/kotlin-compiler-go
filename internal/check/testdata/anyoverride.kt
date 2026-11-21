// The POTENTIALLY_NON_REPORTED_ANNOTATION warnings kotlinc 2.4.21
// reports of the overrides below, as TestWarningsAnyOverride reads them:
// each line ends with a comment naming the warnings it reports, if any.
annotation class Other

@RequiresOptIn(message = "This is a test.")
@Retention(AnnotationRetention.BINARY)
@Target(AnnotationTarget.FUNCTION)
annotation class OptInMarker

class A {
    @Other
    @Deprecated("equals") // POTENTIALLY_NON_REPORTED_ANNOTATION
    @SinceKotlin("1.2") // POTENTIALLY_NON_REPORTED_ANNOTATION
    override fun equals(other: Any?): Boolean {
        return super.equals(other)
    }

    @SinceKotlin("1.3") // POTENTIALLY_NON_REPORTED_ANNOTATION
    override fun hashCode(): Int {
        return super.hashCode()
    }

    @Deprecated("toString") // POTENTIALLY_NON_REPORTED_ANNOTATION
    @OptInMarker // POTENTIALLY_NON_REPORTED_ANNOTATION
    override fun toString(): String {
        return super.toString()
    }

    @Deprecated("other")
    @OptInMarker
    fun test(): Int = 5
}

class B {
    @java.lang.Deprecated // DEPRECATED_JAVA_ANNOTATION POTENTIALLY_NON_REPORTED_ANNOTATION
    override fun hashCode(): Int = 1
    @Deprecated("x")
    fun equals(other: B): Boolean = true
    @Deprecated("y")
    fun toString(x: Int): String = ""
}
interface I {
    @Deprecated("z") // POTENTIALLY_NON_REPORTED_ANNOTATION
    override fun toString(): String
}
data class D(val x: Int) {
    @Deprecated("q") // POTENTIALLY_NON_REPORTED_ANNOTATION
    override fun equals(other: Any?): Boolean = true
}

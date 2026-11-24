// The DSL_MARKER_APPLIED_TO_WRONG_TARGET warnings kotlinc 2.4.21 reports
// of the DSL marker annotations below, as TestWarningsDslMarkerTarget
// reads them: each line ends with a comment naming the warnings it
// reports, if any.
@DslMarker
@Target(AnnotationTarget.CLASS, AnnotationTarget.TYPEALIAS, AnnotationTarget.TYPE, AnnotationTarget.FUNCTION, AnnotationTarget.PROPERTY,
    AnnotationTarget.VALUE_PARAMETER, AnnotationTarget.LOCAL_VARIABLE, AnnotationTarget.CONSTRUCTOR, AnnotationTarget.TYPE_PARAMETER,
    AnnotationTarget.EXPRESSION, AnnotationTarget.FILE, AnnotationTarget.PROPERTY_GETTER, AnnotationTarget.FIELD)
@Retention(AnnotationRetention.SOURCE)
annotation class A1

@A1 class C @A1 constructor(@A1 val p: Int, @A1 q: Int) { // DSL_MARKER_APPLIED_TO_WRONG_TARGET
    @A1 fun m() {} // DSL_MARKER_APPLIED_TO_WRONG_TARGET
    @A1 val x = 1 // DSL_MARKER_APPLIED_TO_WRONG_TARGET
    val y: Int @A1 get() = 1 // DSL_MARKER_APPLIED_TO_WRONG_TARGET
    @get:A1 val z = 1 // DSL_MARKER_APPLIED_TO_WRONG_TARGET
    @A1 object O
    fun <@A1 T> g() {} // DSL_MARKER_APPLIED_TO_WRONG_TARGET
}
@A1 typealias TA = C
@A1 interface I
enum class E { @A1 E1 } // DSL_MARKER_APPLIED_TO_WRONG_TARGET
fun f(x: @A1 C) {
    @A1 val l = 1 // DSL_MARKER_APPLIED_TO_WRONG_TARGET
    @A1 fun loc() {} // DSL_MARKER_APPLIED_TO_WRONG_TARGET
}

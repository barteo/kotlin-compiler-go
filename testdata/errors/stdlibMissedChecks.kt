// Checks that kotlinc 2.4's diagnostics tests expect and kc missed, each
// a case of a check kc has otherwise.

import kotlin.reflect.KProperty

operator fun Any.provideDelegate(x: Any?, p: Int) {} // ERROR second parameter must be of type KProperty<*> or its supertype
operator fun String.provideDelegate(x: Any?, p: KProperty<*>) = this

fun anonymousReceivers() {
    val bag: Int.() -> Unit = fun String.() {} // ERROR expected receiver of type Int
    val ok: Int.() -> Unit = fun Int.() {}
    val wider: Int.() -> Unit = fun Any.() {}
}

@RequiresOptIn
@Target(AnnotationTarget.PROPERTY, AnnotationTarget.FIELD, AnnotationTarget.PROPERTY_GETTER)
annotation class ExperimentalAPI

val numbersB: List<Int>
    @ExperimentalAPI // ERROR opt-in requirement marker 'ExperimentalAPI' can't be used on this target
    field = mutableListOf()

@field:ExperimentalAPI // ERROR opt-in requirement marker 'ExperimentalAPI' can't be used on this target
val numbersC: List<Int> = listOf()

fun readsField() {
    val b: MutableList<Int> = numbersB // ERROR this declaration needs opt-in
}

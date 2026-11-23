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

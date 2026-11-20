class D

val x: Int by D() // ERROR delegated properties need kotlin.reflect.KProperty

// A delegate whose type is a type variable that only a lambda's
// parameter types bound has no getValue while it isn't inferred.
interface HasGetValue {
    operator fun getValue(x: Any?, y: Any?): String = ""
}
fun <T> fromLambda(x: (T) -> Unit): T = null!!

fun bareVariableDelegate() {
    val x by fromLambda { x: HasGetValue -> } // ERROR has no 'getValue' method
}

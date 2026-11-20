// Without -Xmulti-platform, as with kotlinc, the expect and actual
// modifiers are errors, each where it is written.
expect fun foo() // ERROR 'expect' and 'actual' declarations can be used only in multiplatform projects

actual fun foo() {} // ERROR 'expect' and 'actual' declarations can be used only in multiplatform projects

expect class Box { // ERROR 'expect' and 'actual' declarations can be used only in multiplatform projects
    fun get(): Int
}

actual class Box { // ERROR 'expect' and 'actual' declarations can be used only in multiplatform projects
    actual fun get(): Int = 1 // ERROR 'expect' and 'actual' declarations can be used only in multiplatform projects
}

fun ordinary() = Box().get()

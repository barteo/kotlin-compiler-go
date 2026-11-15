// The receiver of ?. is a value: a class without a companion object
// or a package can't be one, as kotlinc's SafeCallOnFakePackage.
fun g() {
    val a = JStatic.FOO
    val b = JStatic?.FOO // ERROR does not have a companion object
    val c = p?.b // ERROR expression expected, but a package name found
    val d = JStatic.Nested()
    val e = p.b()
    val f = WithCompanion?.x
    val g = Obj?.x
}

class WithCompanion {
    companion object {
        val x = 1
    }
}

object Obj {
    val x = 1
}

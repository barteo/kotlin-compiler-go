// The receiver of ?. is a value: a class without a companion object
// or a package can't be one, as kotlinc's SafeCallOnFakePackage.
fun g() {
    val a = JStatic.FOO
    val b = JStatic?.FOO // ERROR does not have a companion object
    val c = p?.b // ERROR expression expected, but a package name found
    val d = JStatic.Nested()
    val e = p.b()
}

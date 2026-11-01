// A private top-level declaration is visible in its file only.

private fun hidden() {}
private fun overloaded(x: Int) {}
fun overloaded() {}
private var counter = 0
private class Hidden {
    class Nested
}
private object HiddenObject {
    val x = 1
}

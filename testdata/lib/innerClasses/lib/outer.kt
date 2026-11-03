// Inner classes of a library's classes, which a program constructs on an
// instance of the outer class or of its subclass.
package second

open class Outer(val x: String) {
    open inner class Inner1 {
        open fun name() = "Inner1 of $x"
    }

    inner class Inner(val test: String) {
        fun show() = "$x.$test"
    }

    inner class Middle(x: String) : Outer(x) {
        inner class Inner2 : Inner1() {
            fun foo() = this@Outer.x + this@Middle.x
        }
    }
}

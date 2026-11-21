// deps: lib
package a.d

import a.b.C
import a.b.Top

open class Derived : Base() {
    override fun top(t: Top) {}

    override fun nested(s: C.Some) {}
}

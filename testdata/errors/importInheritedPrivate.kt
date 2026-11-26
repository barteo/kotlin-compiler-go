// An object doesn't inherit its superclass's private members, so they
// can't be imported from it.

import B.foo // ERROR cannot access 'foo': it is private in 'C'
import B.bar

object B : C<String>()

open class C<T> {
    private var foo: String = "abc"
    var bar: String = "abc"
}

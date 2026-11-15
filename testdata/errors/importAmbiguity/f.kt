// An explicit import, or the file's own package, comes before star
// imports: neither name is ambiguous.
package f

import a.*
import b.*
import a.X

class Some

fun types(x: X, s: Some) {}

val qualified = b.Some.foo + a.Some.foo

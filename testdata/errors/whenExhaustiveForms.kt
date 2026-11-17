// A when without else must still cover every value: an enum's entries
// below a sealed interface all, a !is check only the sealed subclasses
// it isn't related to, and an is check of the subject's type null only
// if it is nullable.
sealed interface Base
interface A : Base
enum class C : Base { X, Y }
object D : Base
sealed class S { object F : S(); open class N : S(); object L : S() }
open class Open
interface Other

fun missingEntry(b: Base) = when (b) { is A -> 1; C.X -> 2; D -> 3 } // ERROR 'when' expression must be exhaustive
fun missingObject(b: Base) = when (b) { is A -> 1; is C -> 2 } // ERROR 'when' expression must be exhaustive
fun negatedSubclass(s: S) = when (s) { is S.F -> 1; !is S.N -> 2 } // ERROR 'when' expression must be exhaustive
fun missingNull(i: Int?) = when (i) { is Int -> 1 } // ERROR 'when' expression must be exhaustive
fun notSealed(o: Open) = when (o) { !is Other -> 1 } // ERROR 'when' expression must be exhaustive
fun supertypeOnly(x: Any) = when (x) { is Base -> 1 } // ERROR 'when' expression must be exhaustive

fun allEntries(b: Base) = when (b) { is A -> 1; C.X -> 2; C.Y -> 3; D -> 4 }
fun withNull(i: Int?) = when (i) { null -> 0; is Int -> 1 }
fun nullableIs(s: String?) = when (s) { is String? -> 1 }
fun negatedBoth(s: S) = when (s) { !is S.N -> 1; is S.N -> 2 }
fun nothing(n: Nothing) = when (n) {}

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

// A value of A checked to be a B is an A & B: a when over it covers A's
// sealed subclasses or B's, not the ones they share alone.
sealed interface IA
sealed interface IB
class SubA : IA
class SubB : IB
class SubAB : IA, IB

fun sharedOnly(x: IA): Int {
    check(x is IB)
    return when (x) { is SubAB -> 1 } // ERROR 'when' expression must be exhaustive
}

fun coversA(x: IA): Int {
    check(x is IB)
    return when (x) { is SubA -> 1; is SubAB -> 2 }
}

// Exhaustive through B's subclasses, though SubB isn't an IA, as K2 has it.
fun coversB(x: IA): Int {
    check(x is IB)
    return when (x) { is SubB -> 1; is SubAB -> 2 } // ERROR check for instance is always 'false'
}

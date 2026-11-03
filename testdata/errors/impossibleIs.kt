// Impossible is checks, bare types whose arguments don't follow from the
// subject, and checks for erased types that kotlinc allows.

open class A
open class B
class F
interface I
open class G<T>
open class H<T>
interface Tr
interface Gen<T>

fun classes(a: A) = a is B // ERROR check for instance is always 'false'
fun negated(a: A) = a !is B // ERROR check for instance is always 'true'
fun final(i: I) = i is F // ERROR check for instance is always 'false'
fun nullable(a: A?) = a is B? // ERROR no common instances but null
fun nullableSubject(a: A?) = a is B // ERROR check for instance is always 'false'
fun generic(g: G<Int>) = g is H<*> // ERROR check for instance is always 'false'
fun <T : G<T>> bound(x: T) = x is H<*> // ERROR check for instance is always 'false'
fun exception(e: Exception) = e is Error // ERROR check for instance is always 'false'
fun string(s: String) = s is Int // ERROR check for instance is always 'false'

fun inWhen(a: A) = when (a) {
    is B -> 1 // ERROR check for instance is always 'false'
    else -> 2
}

// Interfaces, open classes against interfaces, intersections and type
// parameters can have common instances.
fun interfaceCheck(a: A) = a is I
fun openToInterface(i: I) = i is B
fun <T> intersection(x: T) where T : G<T>, T : CharSequence = x is H<*>
fun <T> param(x: T) = x is B
fun anyCheck(x: Any) = x is F
fun subclass(a: A) = a is SubA
open class SubA : A()

fun smartCast(x: Any) {
    if (x is A) {
        println(x is I)
    }
}

fun bareUnrelated(tr: Tr) = tr is Gen // ERROR one type argument expected for Gen
fun bareAs(tr: Tr) = tr as Gen? // ERROR one type argument expected for Gen
fun bareAny(x: Any) = x is Gen // ERROR one type argument expected for Gen
interface SubGen<T> : Gen<T>
fun bareKnown(x: Gen<String>) = x is SubGen

// Erased types that the subject's type determines.
open class BaseTwo<A, B>
open class DerivedWithOne<D> : BaseTwo<D, String>()
fun <T, U> derivedWithOne(a: BaseTwo<T, U>) = a is DerivedWithOne<T>

open class BaseMulti<out A, B>
class SomeMultiDerived<out D> : BaseMulti<D, Any>()
fun multi(t: BaseMulti<String, String>) = t is SomeMultiDerived<Any>

fun <T> self(a: T) = a is T
fun <T : Any> nonNull(a: T?) = a is T
fun <T> nullableParam(a: T?) = a is T // ERROR cannot check for instance of erased type: T

interface Model<T>
interface Node<out T : Any>
open class NodeModel<C : Any> : Model<Node<C>>
fun nested(m: Model<Node<String>>) = m as NodeModel

// Projections in typealias expansions.
interface Out<out T>
typealias TA<K> = (String) -> Out<K>
fun projections(x: Any) {
    x as TA<out Any>
    x as TA<in Any> // ERROR conflicting projection in typealias expansion
}

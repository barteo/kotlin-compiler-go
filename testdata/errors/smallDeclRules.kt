// Declarations kotlinc rejects: consts outside named objects or with a
// getter, private setters of abstract properties, array upper bounds.
class C(const val b: Boolean) { // ERROR const 'val' are only allowed on top level
    const val c = 3 // ERROR const 'val' are only allowed on top level
    companion object {
        const val k = 4
    }
}

const val l = 3
    get // ERROR const 'val' should not have a getter

abstract class My {
    abstract var x: Int
        private set // ERROR private setters are not allowed for abstract properties
    var y: Int = 0
        private set
}

fun <A : Array<Any>> f1() {} // ERROR upper bound of a type parameter cannot be an array
fun <T : IntArray> f4() {} // ERROR upper bound of a type parameter cannot be an array
fun <T : Comparable<T>> ok() {}

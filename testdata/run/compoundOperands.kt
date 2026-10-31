// Compound assignments through an operator function, x op= v as
// x = x.op(v): the target's receiver, or an array element's array and
// indices, are evaluated once.

var log = ""

class A(val value: Int) {
    operator fun plus(other: A): A {
        log += "plus(" + other.value + ");"
        return A(value + other.value)
    }
}

class B(value: Int) {
    var a = A(value)
}

val elements = arrayOf(A(1), A(2))

fun getArray(): Array<A> {
    log += "getArray();"
    return elements
}

fun index(): Int {
    log += "index();"
    return 1
}

val property = B(10)

val b: B
    get() {
        log += "b;"
        return property
    }

fun main() {
    getArray()[index()] += A(3)
    println(elements[1].value)
    b.a += A(20)
    println(property.a.value)
    val local = B(100)
    local.a += A(200)
    println(local.a.value)
    println(log)
}

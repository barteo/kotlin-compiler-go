// An infix call needs an infix function: built-in members and a
// function type's invoke aren't, but the bitwise functions are.
class A(val f: (Int) -> Int)

infix fun A.f(b: Int) = b

infix fun Int.rem(b: Int) = b

class H(val h: (Int) -> Int)

fun infixCalls(a: A, h: H, ext: Int.(Int) -> Unit, s: (Int) -> Unit) {
    println(1 plus 1) // ERROR 'infix' modifier is required on 'plus'
    println(2L times 3L) // ERROR 'infix' modifier is required on 'times'
    println("a" plus "b") // ERROR 'infix' modifier is required on 'plus'
    println("abc" get 0) // ERROR 'infix' modifier is required on 'get'
    println(1 rangeTo 3) // ERROR 'infix' modifier is required on 'rangeTo'
    11 ext 11 // ERROR 'infix' modifier is required on 'invoke'
    s invoke 11 // ERROR 'infix' modifier is required on 'invoke'
    println(h h 1) // ERROR 'infix' modifier is required on 'invoke'
    println(1 shl 2)
    println(1 and 2)
    println(5L ushr 1)
    println(true and false)
    println(true xor false)
    println(a f 1)
    println(7 rem 2)
    println(1.plus(2))
    11.ext(11)
    s(11)
}

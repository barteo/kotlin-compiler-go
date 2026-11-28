// A primitive type has a private constructor without parameters, as
// kotlinc's: it can't be called, and an argument is one too many.
typealias AInt = Int

fun f() {
    Int() // ERROR cannot access 'constructor(): Int': it is private in 'kotlin/Int'
    Boolean() // ERROR cannot access 'constructor(): Boolean'
    Long(1) // ERROR too many arguments for 'constructor(): Long'
    AInt(42) // ERROR too many arguments for 'constructor(): Int'
}

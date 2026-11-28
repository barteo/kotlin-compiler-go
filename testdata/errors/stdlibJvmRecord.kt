// kotlinc: -jvm-target 17
// The checks of kotlinc's FirJvmRecordChecker: a @JvmRecord class is a
// final data class, neither local nor inner, whose constructor's vals,
// at least one, only the last a vararg, are its only fields.

interface I
abstract class Base
val delegate: I = object : I {}
typealias TR = Record

@JvmRecord
data class Ok(val x: Int, val y: String) : I {
    val computed: Int get() = x
    fun f() = x
    companion object { val c = 1 }
    class Nested { val n = 1 }
}

@JvmRecord // ERROR only data classes are allowed to be marked as '@JvmRecord'
class NotData(val x: Int)

@JvmRecord // ERROR only data classes are allowed to be marked as '@JvmRecord'
object O

@JvmRecord // ERROR primary constructor with parameters is required for '@JvmRecord' class
data object DO

@JvmRecord
open class Open(val x: Int) // ERROR '@JvmRecord' class must be final

@JvmRecord
sealed class Sealed(val x: Int) // ERROR '@JvmRecord' class must be final

@JvmRecord
interface Intf // ERROR '@JvmRecord' class must be final

@JvmRecord
enum class E(val x: String) { // ERROR '@JvmRecord' class cannot be an enum
    A("")
}

class Outer {
    @JvmRecord
    inner class Inner(val x: Int) // ERROR '@JvmRecord' class cannot be inner
}

fun local() {
    @JvmRecord // ERROR local '@JvmRecord' classes are prohibited
    data class L(val x: Int)
}

@JvmRecord
data class Vararg(
    vararg val x: String, // ERROR only the last constructor parameter of '@JvmRecord' can be a vararg
    val y: Int,
)

@JvmRecord
data class Vars(
    var a: Int, // ERROR constructor parameter of '@JvmRecord' class must be a 'val'
    val b: Int,
)

@JvmRecord
data class Fields(val a: Int) {
    val x = 0 // ERROR non-constructor properties with backing field in '@JvmRecord' class are prohibited
    val z: Int by lazy { 1 } // ERROR non-constructor properties with backing field in '@JvmRecord' class are prohibited
    lateinit var s: String // ERROR non-constructor properties with backing field in '@JvmRecord' class are prohibited
    val w: Int get() = a
}

@JvmRecord
data class Delegating(val a: Int) : I by delegate // ERROR delegation is prohibited for '@JvmRecord' classes

@JvmRecord
data class ExtendsClass(val a: Int) : Base(), I // ERROR record cannot extend a class

@JvmRecord
data class ExtendsAny(val a: Int) : Any() // ERROR record cannot extend a class

@JvmRecord
data class RecordSuper(val a: Int) : Record() // ERROR classes cannot have explicit 'java.lang.Record' supertype

data class AliasSuper(val a: Int) : TR() // ERROR classes cannot have explicit 'java.lang.Record' supertype

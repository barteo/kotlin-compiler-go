// An object expression's type in a type argument of the type a declaration
// visible outside infers is approximated too, to its supertype or Any:
// Inv<I1> for a private inline function's Inv(object : I1 { }). A private
// function that isn't inline keeps it.
class Inv<T>(val value: T)
interface I1

private inline fun inlineInv(crossinline f: () -> Int) = Inv(object : I1 {
    fun bar(): Int = f()
})

private inline fun inlineInvInv(crossinline f: () -> Int) = Inv(Inv(object {
    fun bar(): Int = f()
}))

private fun privateInv(f: () -> Int) = Inv(object : I1 {
    fun bar(): Int = f()
})

fun publicInv() = Inv(object : I1 {
    fun bar(): Int = 1
})

fun test() {
    inlineInv { 1 }.value.bar() // ERROR unresolved reference
    inlineInvInv { 1 }.value.value.bar() // ERROR unresolved reference
    publicInv().value.bar() // ERROR unresolved reference
    val i: I1 = inlineInv { 1 }.value
    privateInv { 1 }.value.bar()
}

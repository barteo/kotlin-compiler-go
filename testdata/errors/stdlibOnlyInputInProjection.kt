// An argument whose type has an in projection among its type arguments is
// captured in K2, and the capture is never what a call infers: contains
// of such an argument can't infer T from it, unless the receiver's
// element type is that type.

class Inv<T>

fun test(a: Inv<in Number>, b: Inv<out Number>, c: Comparator<in String>) {
    listOf(Inv<Number>()).contains(a) // ERROR must be mentioned in input types
    listOf(Inv<Any>()).contains(a) // ERROR must be mentioned in input types
    listOf(Inv<Number>()).contains(b)
    val cs: List<Comparator<in String>> = listOf()
    cs.contains(c)
    val ins: List<Inv<in Number>> = listOf()
    ins.contains(a)
    ins.contains(Inv<Number>())
}

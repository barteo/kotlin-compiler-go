// The warnings kotlinc 2.4.21 reports of type arguments out of their
// bounds in a typealias's expansion that only become errors later: each
// line ends with a comment naming the warnings it reports, if any.
class NumColl<T : Collection<Number>>
typealias NL<T2> = NumColl<List<T2>>
typealias MMMM<A3> = NL<A3>
val nested = NL<Any>() // UPPER_BOUND_VIOLATED_IN_TYPEALIAS_EXPANSION_DEPRECATION_WARNING
val within = MMMM<Int>()

// A class literal of a generic typealias written without arguments.
class Inv<S>
class C<X, Y : X>
typealias TA<T> = C<Inv<String>, Inv<T>>
typealias TA2<T> = C<Int, List<T>>
typealias TA3<T> = C<List<T>, List<String>>
typealias TA5<T> = C<MutableList<T>, List<T>>
fun literals() {
    println(TA::class) // UPPER_BOUND_VIOLATED_IN_LHS_OF_CLASS_LITERAL_WARNING
    println(TA2::class) // UPPER_BOUND_VIOLATED_IN_LHS_OF_CLASS_LITERAL_WARNING
    println(TA3::class)
    println(TA5::class) // UPPER_BOUND_VIOLATED_IN_LHS_OF_CLASS_LITERAL_WARNING
}

// A constructor call through a typealias whose arguments it infers.
class Foo<A : Number>
class Bar<B : CharSequence>
class Hr<A, B, C, D>(val a: A, val b: B)
typealias Test<A, B> = Hr<A, B, Foo<A>, Bar<B>>
typealias Bas<T> = Hr<T, T, Foo<T>, Bar<T>>
val inferred = Test(1, "")
val inferred2 = Test(1, 2) // UPPER_BOUND_VIOLATED_DEPRECATION_WARNING
val inferred3 = Bas(1, 1) // UPPER_BOUND_VIOLATED_DEPRECATION_WARNING
val inArgument = listOf(Test(1, 2)) // UPPER_BOUND_VIOLATED_DEPRECATION_WARNING

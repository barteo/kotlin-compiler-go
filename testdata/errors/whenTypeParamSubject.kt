// A when over a value of a type parameter's type is exhaustive if it
// covers one of the parameter's bounds, as in K2: the entries of a
// T : MyEnum, the subclasses of a T : Sealed, after the values an
// earlier check rules out too.
enum class MyEnum { A, B, C }

sealed interface Sealed {
    object A : Sealed
    object B : Sealed
    object C : Sealed
}

fun <T : MyEnum> all(e: T): Int = when (e) {
    MyEnum.A -> 1
    MyEnum.B -> 2
    MyEnum.C -> 3
}

fun <T : MyEnum> ruledOut(e: T): Int {
    if (e == MyEnum.A) return 1
    return when (e) {
        MyEnum.B -> 2
        MyEnum.C -> 3
    }
}

fun <T : MyEnum> nullable(e: T?): Int {
    if (e == null) return 0
    if (e == MyEnum.A) return 1
    return when (e) {
        MyEnum.B -> 2
        MyEnum.C -> 3
    }
}

fun <T : Sealed> sealed(x: T): Int {
    if (x is Sealed.A) return 1
    return when (x) {
        Sealed.B -> 2
        Sealed.C -> 3
    }
}

fun <T : MyEnum> missing(e: T): Int {
    return when (e) { // ERROR 'when' expression must be exhaustive
        MyEnum.B -> 2
        MyEnum.C -> 3
    }
}

fun <T : MyEnum> nullNotCovered(e: T?): Int = when (e) { // ERROR 'when' expression must be exhaustive
    MyEnum.A -> 1
    MyEnum.B -> 2
    MyEnum.C -> 3
}

// The smart casts where a try's body and catch blocks end hold after it,
// with those of its finally block on top, and a break out of the try
// jumps with the finally block's.
interface A { fun aaa(): String }
interface B { fun bbb(): String }
interface C { fun ccc(): String }
class Abc : A, B, C {
    override fun aaa() = "a"
    override fun bbb() = "b"
    override fun ccc() = "c"
}

fun withoutCatch(s: Any): Int {
    try {
        s as String
    } finally {
        println("finally")
    }
    return s.length
}

fun returnInCatch(x: Any?): String {
    try {
        x as A
    } catch (e: Exception) {
        return "caught"
    } finally {
        println("finally")
    }
    return x.aaa()
}

fun castInBoth(s: Any): Int {
    try {
        s as String
    } catch (e: Exception) {
        s as String
    }
    return s.length
}

fun bodyAndFinally(x: Any?): String {
    try {
        x as C
    } finally {
        try {
            x as A
        } catch (e: Exception) {
            return "caught"
        }
        print(x.aaa())
    }
    return x.aaa() + x.ccc()
}

fun breakThroughFinally(x: Any?): String {
    while (true) {
        try {
            x as A
            break
        } finally {
            x as B
        }
    }
    return x.aaa() + x.bbb()
}

fun main() {
    println(withoutCatch("abc"))
    println(returnInCatch(Abc()))
    println(returnInCatch("not A"))
    println(castInBoth("ab"))
    println(bodyAndFinally(Abc()))
    println(breakThroughFinally(Abc()))
}

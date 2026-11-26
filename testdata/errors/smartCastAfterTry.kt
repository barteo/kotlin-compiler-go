// What a try's blocks don't tell after it, as with kotlinc: a cast in the
// body when a catch block ends without it, nor one in a catch block that
// jumps, and in the finally block, none of the body's or catch blocks'.
interface A { fun aaa() }
interface B { fun bbb() }

fun castInTry(s: Any) {
    try {
        s as String
    } catch (e: Exception) {
    }
    s.length // ERROR unresolved reference: length
}

fun returnInCatch(x: Any) {
    try {
        x as A
    } catch (e: Exception) {
        x as B
        return
    } finally {
        x.aaa() // ERROR unresolved reference: aaa
    }
    x.aaa()
    x.bbb() // ERROR unresolved reference: bbb
}

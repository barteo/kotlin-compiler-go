import lib.*

fun test(c: Ctx) {
    c.o // ERROR needs opt-in
    c.d // ERROR 'd' is deprecated with level ERROR
    c.plain
}

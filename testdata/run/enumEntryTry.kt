// A try in the arguments of an enum entry's constructor call: its handler
// starts with an empty stack, so the arguments are evaluated first, as
// kotlinc does, and the entry is made after them.
fun parse(s: String): Int = if (s == "bad") throw IllegalArgumentException(s) else s.length

inline fun guarded(f: () -> Int): Int = try { f() } catch (e: IllegalArgumentException) { -1 }

enum class E(val a: Int, val b: String = "b") {
    A(try { parse("abc") } catch (e: IllegalArgumentException) { 0 }),
    B(try { parse("bad") } catch (e: IllegalArgumentException) { 99 }, "caught"),
    C(guarded { parse("bad") }, try { "t" } finally { println("finally C") }),
    D(1 + try { try { parse("bad") } finally { println("inner D") } } catch (e: Exception) { 41 }),
    F(a = try { 7 } catch (e: Exception) { 8 }),
    G(2);

    constructor(s: String) : this(try { parse(s) } catch (e: IllegalArgumentException) { -2 }, s)
}

enum class S(val s: String) {
    X("x"), Y(try { E.B.b } catch (e: Exception) { "?" });
    constructor() : this("none")
}

fun main() {
    for (e in E.values()) println("$e ${e.a} ${e.b} ${e.ordinal}")
    for (s in S.values()) println("$s ${s.s}")
}

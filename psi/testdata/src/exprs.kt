fun exprs(a: Int, b: Int?, s: String, l: List<Int>) {
    val x = a + b!! * 2 - (a / 3) % 4
    val y = b ?: 0
    val z = s?.length?.plus(1) ?: -1
    val r = 1..10
    val ru = 0..<10
    val c = a in r && a !in l || a is Int && s !is CharSequence
    val cast = s as Any
    val safe = s as? Int
    var i = 0
    i += 1; i -= 1; i *= 2; i /= 2; i %= 3
    i++; i--; ++i; --i
    val neg = -a
    val not = !c
    val cmp = a < b!! && a <= 2 && a > 1 && a >= 0 && a == 1 && a != 2 && a === a && a !== a
    val call = foo(1, x = 2, *arr)
    val trailing = run { it }
    val both = l.map({ it + 1 }) { it }
    val gen = listOf<Int>(1, 2)
    val idx = l[0] + m[1, 2]
    val ref = ::foo
    val cref = String::length
    val cls = C::class
    val jcls = C::class.java
    val lambda = { p: Int, q -> p + q }
    val destr = { (k, v): Pair<Int, Int> -> k }
    val anon = fun(x: Int): Int { return x }
    val obj = object : Runnable { override fun run() {} }
    val thiz = this@exprs
    val sup = super<Any>.hashCode()
    label@ for (e in l) { if (e == 0) continue@label else break@label }
    while (i > 0) i--
    do { i++ } while (i < 10)
    for ((k, v) in map) println("$k=$v")
    val w = when (a) {
        0, 1 -> "small"
        in 2..5 -> "mid"
        is Int -> "int"
        !in r -> "out"
        else -> { "big" }
    }
    val w2 = when { a > 0 -> 1; else -> 2 }
    val w3 = when (val q = a) { 1 -> q else -> 0 }
    val t = try { risky() } catch (e: Exception) { null } finally { done() }
    val ifx = if (a > 0) 1 else if (a < 0) -1 else 0
    throw IllegalStateException("x")
    return
}
fun strings(n: Int) {
    val a = "plain"
    val b = "esc \n \t \" \\ \$ A"
    val c = "tmpl $n ${n + 1} ${"nested ${n}"}"
    val d = """raw
        $n ${n * 2} "quotes" \n
    """.trimIndent()
    val ch = 'a'
    val ech = '\n'
    val uch = 'ሴ'
    val nums = listOf(1, 0x1F, 0b101, 1_000, 1L, 1.0, 1e10, 1.5f, .5, 1u, 0xFFuL)
    val bools = true && false
    val nul = null
}
fun generics() {
    val a = foo<List<Map<String, Int>>>()
    if (a < b && c > d) {}
    val p = a < b
    val q = f(a < b, c > d)
}

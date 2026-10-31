// Varargs of unsigned integers, which are unsigned arrays (UIntArray),
// the standard library's inline-only uintArrayOf and its kin, spreads of
// unsigned arrays and loops over them.

fun sum(vararg xs: UInt): UInt {
    var s = 0u
    for (x in xs) s += x
    return s
}

fun count(label: String, vararg bytes: UByte) = "$label: ${bytes.size}"

fun main() {
    val a = uintArrayOf(1u, 2u, 3u)
    println(a.toList())
    println(ubyteArrayOf(255u, 1u).toList())
    println(ushortArrayOf().size)
    println(ulongArrayOf(ULong.MAX_VALUE)[0])
    println(sum(4u, 5u))
    println(sum(*a))
    println(sum(*a, 10u, *a))
    println(sum())
    println(count("bytes", 1u, 2u, 3u))
    val b = uintArrayOf(*a, 4u)
    b[0] = 9u
    println("${a.toList()} ${b.toList()}")
    for (x in ulongArrayOf(7uL, 8uL)) print("$x ")
    println()
}

// Compound assignments and ++ of an element of an array whose get and
// set are operator extensions taking other indices: x.set(i, x.get(i) op
// v), with the array and the indices evaluated once.
operator fun Array<String>.get(i: Int, j: Int) = this[i + j]
operator fun Array<String>.set(i: Int, j: Int, v: String) {
    this[i + j] = v
}

operator fun IntArray.get(i: Long) = this[i.toInt()]
operator fun IntArray.set(i: Long, v: Int) {
    this[i.toInt()] = v
}

var log = ""

fun <T> logged(s: String, x: T): T {
    log += s
    return x
}

fun main() {
    val s = Array(2) { "" }
    s[1, -1] = "a"
    s[2, -1] += "b"
    logged("s", s)[logged("i", 0), logged("j", 0)] += "c"
    println("${s[0, 0]} ${s[0, 1]} $log")
    val l = IntArray(2)
    l[1L] = 4
    l[1L] += 6
    l[0L]++
    ++l[0L]
    l[1L] *= 2
    println("${l[0L]} ${l[1L]} ${l[0]}")
    val a = IntArray(1)
    a[0] += 3
    a[0]++
    println(a[0])
}

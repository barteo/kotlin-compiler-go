class C {
    val arr = IntArray(2)
    val narr: IntArray? = null
    val d: D? = null

    fun nf(): IntArray? = null
}

class D {
    val arr = IntArray(2)
}

fun read(c: C?): Int? = c?.arr[0]

fun readNullable(c: C?): Int? = c?.narr[0] // ERROR only safe (?.) or non-null asserted (!!.) calls are allowed on a nullable receiver of type IntArray?

fun readCall(c: C?): Int? = c?.nf()[0] // ERROR only safe (?.) or non-null asserted (!!.) calls are allowed on a nullable receiver of type IntArray?

fun writeNullable(c: C?) {
    c?.narr[0] = 1 // ERROR only safe (?.) or non-null asserted (!!.) calls are allowed on a nullable receiver of type IntArray?
    c?.narr[0] += 1 // ERROR only safe (?.) or non-null asserted (!!.) calls are allowed on a nullable receiver of type IntArray?
    c?.narr[0]++ // ERROR only safe (?.) or non-null asserted (!!.) calls are allowed on a nullable receiver of type IntArray?
}

fun afterChain(c: C?) {
    c?.d.arr[0] = 1 // ERROR only safe (?.) or non-null asserted (!!.) calls are allowed on a nullable receiver of type D?
}

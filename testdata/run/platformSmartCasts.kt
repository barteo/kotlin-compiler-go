// Equality with a value of a platform type, such as a Java static field,
// smart casts the other side and the receivers of its safe calls to
// non-null.

import java.io.File

class Req(val action: String?, val data: String?)

fun whenSafeCall(r: Req?): String? =
    when (r?.action) {
        File.separator -> r.data
        else -> null
    }

fun whenJavaGetter(f: File?): String? =
    when (f?.name) {
        File.separator -> f.path
        "a.txt" -> f.parent
        else -> null
    }

fun notEqualReturn(r: Req?): String? {
    if (r?.action != File.separator) return null
    return r.data
}

fun equalReversed(f: File?): Int =
    if (File.pathSeparator == f?.name) f.path.length else -1

fun localSubject(s: String?): Int =
    when (s) {
        File.separator -> s.length
        else -> 0
    }

fun localEquality(s: String?): Int = if (s == File.separator) s.length else -1

fun main() {
    println(whenSafeCall(Req("/", "data")))
    println(whenSafeCall(Req("x", "data")))
    println(whenSafeCall(null))
    println(whenJavaGetter(File("dir/a.txt")))
    println(whenJavaGetter(null))
    println(notEqualReturn(Req("/", "ok")))
    println(notEqualReturn(null))
    println(equalReversed(File(":")))
    println(equalReversed(null))
    println(localSubject("/"))
    println(localSubject(null))
    println(localEquality("/"))
    println(localEquality("x"))
}

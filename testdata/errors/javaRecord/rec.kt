// A Java record's components are properties to Kotlin; its other
// methods are not.

fun use(r: R<Int>, s: R<*>) {
    val a: Int = r.x
    val b: String = r.name
    val c: Int = r.x()
    val d: Any? = s.x
    r.other()
    r.other // ERROR unresolved
}

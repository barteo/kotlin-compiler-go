// The warnings kotlinc 2.4.21 reports of the explicit backing fields
// below, as TestWarningsBackingField reads them: each line ends with a
// comment naming the warnings it reports, if any.
class K {
    val a: Int
        field = 42 // REDUNDANT_EXPLICIT_BACKING_FIELD
    val b: List<String>
        field = mutableListOf("a")
    val c: (Int) -> Unit
        field: Int.() -> Unit = {} // REDUNDANT_EXPLICIT_BACKING_FIELD
    val d: suspend () -> Unit
        field: suspend () -> Unit = {} // REDUNDANT_EXPLICIT_BACKING_FIELD
    val e: Any
        field: suspend () -> Unit = {}
    val f: Number
        field: Int = 1
    val g: String?
        field: String? = null // REDUNDANT_EXPLICIT_BACKING_FIELD
    val h: CharSequence
        field = "s"
}
val top: Int
    field = 1 // REDUNDANT_EXPLICIT_BACKING_FIELD

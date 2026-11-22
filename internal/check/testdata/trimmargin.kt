// The warnings kotlinc 2.4.21 reports of the calls of trimMargin below, as
// TestWarningsTrimMargin reads them: each line ends with a comment naming
// the warnings it reports, if any.
const val blank = " "
fun use(s: String) {}
val s1 = "".trimMargin(" ") // TRIM_MARGIN_BLANK_PREFIX
val s2 = "".trimMargin(blank)
val s3 = "".trimMargin( // TRIM_MARGIN_BLANK_PREFIX
    """

    """)
val s4 = "a".trimMargin("") // TRIM_MARGIN_BLANK_PREFIX
val s5 = "a".trimMargin("\t") // TRIM_MARGIN_BLANK_PREFIX
val s6 = "a".trimMargin(marginPrefix = "  ") // TRIM_MARGIN_BLANK_PREFIX
val s7 = "a".trimMargin("|")
val s8 = "a".trimMargin()
val s9 = "a".trimMargin(" ${""}")
val s10 = "a".trimMargin(" " + " ")
val s11 = "a".trimMargin((" ")) // TRIM_MARGIN_BLANK_PREFIX
fun f(x: String, y: String?) {
    use(x.trimMargin(" ")) // TRIM_MARGIN_BLANK_PREFIX
    use(y?.trimMargin(" ") ?: "") // TRIM_MARGIN_BLANK_PREFIX
    with(x) { use(trimMargin(" ")) } // TRIM_MARGIN_BLANK_PREFIX
    use(x.trimMargin("\n")) // TRIM_MARGIN_BLANK_PREFIX
    use(x.trimMargin(" ")) // TRIM_MARGIN_BLANK_PREFIX
    use(x.replaceIndentByMargin(" ", " "))
    use(x.trimIndent())
    @Suppress("TRIM_MARGIN_BLANK_PREFIX")
    use(x.trimMargin(" "))
    val r = String::trimMargin
}

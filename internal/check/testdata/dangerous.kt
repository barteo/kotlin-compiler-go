// DANGEROUS_CHARACTERS: a declaration's name with ?*"|%, characters that
// file names can't have on Windows, or that expand variables there.
package `a?b`.c // DANGEROUS_CHARACTERS

class `A?B`<`T*`>(`p%`: Int, val `q%`: Int) { // DANGEROUS_CHARACTERS
    fun `m|`(`x"`: Int) = `x"` // DANGEROUS_CHARACTERS
    val `v?` = 1 // DANGEROUS_CHARACTERS
    constructor(`s*`: String) : this(1, 2) // DANGEROUS_CHARACTERS
    fun plain(x: Int) = `v?` + x
}

enum class E { `E?` } // DANGEROUS_CHARACTERS

typealias `TA?` = Int // DANGEROUS_CHARACTERS

fun `?*"|%`(): Int { // DANGEROUS_CHARACTERS
    val `?` = 0 // DANGEROUS_CHARACTERS
    val (`d?`, `e?`) = Pair(1, 2) // DANGEROUS_CHARACTERS
    for (`i?` in 0..1) {} // DANGEROUS_CHARACTERS
    try {} catch (`ex?`: Exception) {} // DANGEROUS_CHARACTERS
    val f = { `l?`: Int -> `l?` } // DANGEROUS_CHARACTERS
    val g = fun(`an?`: Int) = `an?` // DANGEROUS_CHARACTERS
    class `Loc?` // DANGEROUS_CHARACTERS
    fun `lf?`() {} // DANGEROUS_CHARACTERS
    val o = object { val `ov?` = 1 } // DANGEROUS_CHARACTERS
    val r = ::`lf?`
    return `?` + `d?` + `e?` + f(1) + g(2) + o.`ov?`
}

val `"a"+"b"` = "c" // DANGEROUS_CHARACTERS

var w: Int = 0
    set(`sv?`) { field = `sv?` } // DANGEROUS_CHARACTERS

interface I { fun <`U?`> u() } // DANGEROUS_CHARACTERS

@Suppress("DANGEROUS_CHARACTERS")
fun `quiet?`() {}

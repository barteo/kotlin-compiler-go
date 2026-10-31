// An internal inline function's copies in other files call the private
// members it calls through accessors, which this file's classes have.

internal inline fun foo(): String = bar()

private fun bar(): String = "bar"

class C {
    internal inline fun fi(): String = bi()
    private fun bi(): String = "bi"
}

private fun dex(): String = "dex"

class CC {
    internal inline fun fx(): String = dex()
}

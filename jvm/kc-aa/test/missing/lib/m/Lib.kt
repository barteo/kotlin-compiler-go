package m

// Lib's class file refers to Missing, which build.sh leaves off the class
// path, as an Android library's implementation dependency is off its
// consumers'.
class Lib {
    fun use(m: Missing) {}
    fun make(): Missing? = null
    fun plain(x: Int) {}
    val held: Missing? = null
}

// kotlinc copies the fields of a companion object's properties into its
// class, renaming one named as a field of the class with $1: the backend
// reports one that clashes. It runs only when the frontend reports no
// error, so this is a file of its own.
class Copied {
    companion object {
        val X = 1
        val `X$1` = 1 // ERROR platform declaration clash
        val Y = 1
        val `Y$1` = "s"
    }
    val X = 1 // ERROR platform declaration clash
    val Y = 1
}

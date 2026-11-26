package sample

class Box(val size: Int, var title: String, hidden: Int) {
    val count: Int = size
    var label = title
    fun read(key: Int): Int = key
    fun read(key: String) = key
}
fun String.tag(value: Int): String = this
val String.lengthCopy: Int get() = length

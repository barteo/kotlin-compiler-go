fun main() {
    val r = lib.geometry.Rect(left = 1f, top = 2f, right = 3f, bottom = 4f)
    println(r)
    println(lib.geometry.Rect(1f, 2f, 5f, 6f))
    println(lib.geometry.Rect(lib.geometry.Point(1f, 1f), 2f))
}

open class Model {
    val tag = "model"
}

val Model.scope: String get() = "scope of $tag"

class Mapper {
    fun map(x: Int) = x * 2
}

class Screen : Model() {
    private val mapper = Mapper()

    fun inWith() = with(mapper) { scope + " " + map(2) }

    fun <T> List<T>.described() = scope + " " + size
}

fun main() {
    val screen = Screen()
    println(screen.inWith())
    with(screen) { println(listOf(1, 2).described()) }
}

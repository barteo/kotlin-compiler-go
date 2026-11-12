// kotlinc: -jvm-default=enable
annotation class Marked(val name: String = "")

annotation class Arg

class Holder

interface WithDefaults {
    @get:Marked("holder")
    val holder: Holder
        get() = Holder()

    @Marked
    fun twice(@Arg n: Int): Int = n * 2

    fun Int.plus(@Arg other: Int): Int = this + other
}

class Impl : WithDefaults

fun main() {
    for (m in Impl::class.java.declaredMethods.sortedBy { it.name }) {
        val anns = m.annotations.map { it.annotationClass.simpleName }
        val params = m.parameterAnnotations.map { ps -> ps.map { it.annotationClass.simpleName } }
        println("${m.name} bridge=${m.isBridge} $anns $params")
    }
    println(Impl().twice(3))
}

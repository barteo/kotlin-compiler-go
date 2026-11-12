import lib.WithDefaults

class Impl : WithDefaults

fun main() {
    for (m in Impl::class.java.declaredMethods.sortedBy { it.name }) {
        val anns = m.annotations.map { it.annotationClass.simpleName }
        val params = m.parameterAnnotations.map { ps -> ps.map { it.annotationClass.simpleName } }
        println("${m.name} bridge=${m.isBridge} $anns $params")
    }
    @Suppress("DEPRECATION")
    println(Impl().twice(3))
}

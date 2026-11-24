import cfg.*

fun main() {
    println(named())
    try {
        println(name)
    } catch (e: UninitializedPropertyAccessException) {
        println(e.message)
    }
    name = "kc"
    println("${named()} $name")
    name += "!"
    println(name)
    println(owned())
    claim("me")
    println("${owned()} $owner")
    secret = "s"
    other = listOf(1, 2)
    println(describe())
    // As Java sees them: the facade's accessors, the part's fields.
    val facade = Class.forName("cfg.Config")
    println(facade.getMethod("getName").invoke(null))
    println(facade.declaredFields.map { it.name }.sorted())
    println(facade.declaredMethods.map { it.name }.filter { "$" !in it }.sorted())
    val part = Class.forName("cfg.Config__ConfigKt")
    println(part.declaredFields.map { java.lang.reflect.Modifier.toString(it.modifiers) + " " + it.name }.sorted())
}

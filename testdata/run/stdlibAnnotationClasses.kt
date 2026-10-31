import kotlin.reflect.KClass

annotation class Ref(val type: KClass<*>, val types: Array<KClass<out Number>>)

@Ref(String::class, [Int::class, Long::class])
class User

fun main() {
    val methods = Ref::class.java.declaredMethods.map { it.name + "(): " + it.returnType.name }.sorted()
    for (m in methods) {
        println(m)
    }
    println(Ref::class.java.isAnnotation)
    println(User::class.simpleName)
}

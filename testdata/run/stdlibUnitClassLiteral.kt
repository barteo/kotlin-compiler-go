import kotlin.reflect.KClass

annotation class Key(val value: KClass<*>)

annotation class Keys(vararg val value: KClass<*>)

@Key(Unit::class)
class Plain

@Key(kotlin.Unit::class)
class Qualified

@Keys(Unit::class, kotlin.Unit::class, Nothing::class, Int::class, IntArray::class, Array<Unit>::class)
class Several

fun main() {
    println(Plain::class.java.getAnnotation(Key::class.java).value.java.name)
    println(Qualified::class.java.getAnnotation(Key::class.java).value.java.name)
    println(Several::class.java.getAnnotation(Keys::class.java).value.joinToString { it.java.name })
    println(Plain::class.java.getAnnotation(Key::class.java).value == Unit::class)
}

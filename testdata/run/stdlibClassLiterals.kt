open class Animal
class Dog : Animal()
class Cat : Animal()

fun kind(a: Animal): String = a::class.simpleName ?: "?"

fun main() {
    println(Dog::class.simpleName)
    println(Dog::class.qualifiedName)
    println(Dog::class.java.simpleName)
    println(String::class.simpleName)
    println(Int::class.simpleName)
    println(List::class.simpleName)
    val animals = listOf(Dog(), Cat(), Dog())
    println(animals.map { kind(it) })
    println(animals.count { it::class == Dog::class })
    println(Animal::class.isInstance(animals[0]))
    println(Cat::class.isInstance(animals[0]))
    val counts = HashMap<kotlin.reflect.KClass<out Animal>, Int>()
    for (a in animals) counts[a::class] = (counts[a::class] ?: 0) + 1
    println(counts[Dog::class])
    println(counts[Cat::class])
    println(Dog::class.java == animals[0]::class.java)
    println(Dog::class.java.kotlin.simpleName)
}

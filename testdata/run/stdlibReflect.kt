// kotlin-reflect reads kc's metadata: the Kotlin view of the program's
// classes, their members' names, types, nullability and defaults.
import kotlin.reflect.full.*

data class Point(val x: Int, val y: Int?)

sealed class Shape {
    class Circle(val r: Double) : Shape()
    object Empty : Shape()
}

enum class Color { RED, GREEN }

open class Account(val owner: String, var balance: Long = 0) {
    fun deposit(amount: Long, note: String? = null): Long {
        balance += amount
        return balance
    }
    open fun describe(): String = owner
    internal fun audit(): List<String> = listOf()
}

object Bank {
    const val NAME = "kc"
    val accounts: MutableList<Account> = mutableListOf()
}

fun main() {
    println(Point::class.isData.toString() + " " + Shape::class.isSealed + " " + Shape::class.isAbstract)
    println(Shape::class.sealedSubclasses.map { it.simpleName ?: "" }.sorted())
    println(Account::class.declaredMemberProperties.map { it.name + ": " + it.returnType }.sorted())
    println(Account::class.declaredMemberFunctions.map { f ->
        f.name + f.parameters.drop(1).map { it.name + ": " + it.type + (if (it.isOptional) "?=" else "") } + ": " + f.returnType
    }.sorted())
    println(Account::class.primaryConstructor!!.parameters.map { it.name + (if (it.isOptional) " optional" else "") })
    println(Account::class.isOpen.toString() + " " + Account::class.declaredMemberFunctions.first { it.name == "describe" }.isOpen)
    println(Bank::class.objectInstance === Bank)
    println(Bank::class.declaredMemberProperties.map { it.name + ": " + it.returnType }.sorted())
    println(Color::class.java.isEnum.toString() + " " + Point::class.declaredMemberProperties.first { it.name == "y" }.returnType.isMarkedNullable)
}

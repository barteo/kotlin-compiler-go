// Property references whose expected type is one of kotlin.reflect's, or
// that have no expected function type, are KProperty objects: they have a
// name, get, and for a var's, set.
import kotlin.reflect.KFunction
import kotlin.reflect.KMutableProperty0
import kotlin.reflect.KMutableProperty1
import kotlin.reflect.KProperty
import kotlin.reflect.KProperty0
import kotlin.reflect.KProperty1

class Money(val currencyCode: String, var amount: Int) {
    var audited: Int = 0
        private set
}

class Box<T>(val value: T)

var total = 3

val Money.doubled: Int get() = amount * 2

var Money.cents: Int
    get() = amount * 100
    set(v) { amount = v / 100 }

fun <T, G> read(p: KProperty1<T, G>, of: T): G = p.get(of)

fun describe(p: KProperty<*>): String = p.name + (if (p is KMutableProperty1<*, *> || p is KMutableProperty0<*>) " (var)" else "")

// Overloads that take a name, a property or a setter function, as
// kotlinfixture's property.
class Config {
    val rules = mutableListOf<String>()

    fun <G> property(name: String, generator: () -> G) {
        rules += "name $name=${generator()}"
    }

    fun <T, G> property(property: KProperty1<T, G>, generator: () -> G) {
        rules += "property ${property.name}=${generator()}"
    }

    fun <G> property(function: KFunction<Unit>, generator: () -> G) {
        rules += "function ${function.name}=${generator()}"
    }
}

fun config(block: Config.() -> Unit): List<String> = Config().apply(block).rules

inline fun <reified T, G> qualified(p: KProperty1<T, G>): String = T::class.java.simpleName + "." + p.name

// In an initializer, a class's method and a lambda.
val codeRef: KProperty1<Money, String> = Money::currencyCode

class Ledger(val entries: List<Money>) {
    var label = "ledger"

    fun codes(): List<String> = entries.map { read(Money::currencyCode, it) }

    fun labelRef(): KMutableProperty0<String> = this::label
}

fun main() {
    val m = Money("GBP", 1)

    val code: KProperty1<Money, String> = Money::currencyCode
    println(code.name + " " + code.get(m))

    val amount: KMutableProperty1<Money, Int> = Money::amount
    amount.set(m, 5)
    println(read(Money::amount, m))
    println(read(Money::currencyCode, m).lowercase())

    // Bound references and top-level properties.
    val bound: KProperty0<Int> = m::amount
    m.amount = 6
    println(bound.get())
    val mutableBound: KMutableProperty0<Int> = m::amount
    mutableBound.set(7)
    println(m.amount)
    val top: KMutableProperty0<Int> = ::total
    top.set(top.get() + 1)
    println(total)

    // Extension properties.
    println(read(Money::doubled, m))
    val cents: KMutableProperty1<Money, Int> = Money::cents
    cents.set(m, 900)
    println(m.amount)

    // A generic class's.
    println(read(Box<String>::value, Box("boxed")))

    // A var with a private setter is a KProperty1.
    println(describe(Money::audited) + ", " + describe(Money::amount) + ", " + describe(::total) + ", " + describe(Money::currencyCode))

    // Equality is that of kotlinc's references.
    val a: KProperty1<Money, Int> = Money::amount
    val b: KProperty1<Money, Int> = Money::amount
    println(a == b)

    println(qualified(Money::amount))
    println(codeRef.get(m))
    val ledger = Ledger(listOf(m, Money("EUR", 2)))
    println(ledger.codes())
    ledger.labelRef().set("book")
    println(ledger.label)

    // Without an expected type.
    val untypedTop = ::total
    untypedTop.set(untypedTop.get() * 2)
    println(untypedTop.name + " " + untypedTop() + " " + total)
    val untypedMember = Money::amount
    untypedMember.set(m, 11)
    println(untypedMember.name + " " + untypedMember.get(m) + " " + untypedMember(m))
    val untypedBound = m::currencyCode
    println(untypedBound.get())
    println((Money::doubled).get(m))
    println(::total == ::total)
    println(listOf(m).map(Money::currencyCode))

    // Overloads, and a lambda whose type the property's gives.
    println(config {
        property(Money::currencyCode) { "USD" }
        property("amount") { 2 }
        property(Money::amount) { 3 }
    })
}

import kotlin.reflect.KMutableProperty1
import lib.Price
import lib.build

class Money(val currencyCode: String, var amount: Int) {
    companion object {
        const val DEFAULT_CURRENCY = "GBP"
    }
}

fun main() {
    println(build {
        property(Money::currencyCode) { Money.DEFAULT_CURRENCY }
        property(Money::amount) { seed * 10 }
        property<Money, Int>("amount") { 7 }
        // The library's class's properties.
        property(Price::currency) { "EUR" }
        property(Price::cents) { seed }
    })
    val cents: KMutableProperty1<Price, Int> = Price::cents
    val p = Price("EUR", 1)
    cents.set(p, 250)
    println(cents.name + " " + cents.get(p) + " " + p.cents)
}

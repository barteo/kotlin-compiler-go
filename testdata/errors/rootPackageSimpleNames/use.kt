package use

fun use() {
    val nested = Klass.Nested()
    val one = Klass.companionFunction()
    val klass: Klass? = null // ERROR unresolved or unsupported type: Klass
    Klass() // ERROR unresolved reference: Klass
    val objekt = Objekt // ERROR unresolved reference: Objekt
    KlassAlias() // ERROR unresolved reference: KlassAlias
}

// As in K2 (KT-69985), a file of another package finds the root package's
// classes and typealiases as qualifiers, though not by their simple names.
package app

fun main() {
    val nested: Klass.Nested = Klass.Nested()
    println(nested.nestedFunction())
    println(Klass.NestedObject.nestedObjectFunction())
    println(Klass.companionFunction())
    println(Klass.Companion.companionFunction())
    println(Objekt.objektFunction())
    println(Objekt.NAME)
    println(Objekt.ObjektNested())
    println(KlassAlias.companionFunction())
    println(ObjektAlias.objektFunction())
    val companion: Klass.Companion = Klass.Companion
    println(companion.companionFunction())
}

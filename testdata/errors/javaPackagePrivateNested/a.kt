// A Java superclass's package-private nested classes are inherited by
// simple name in its package, as K2 has it, through Kotlin and Java
// subclasses; not its private ones, nor in another package.
package p

open class D : C()

class X : D() {
    init {
        BS()
        BI()
        BP() // ERROR unresolved reference: BP
    }
}

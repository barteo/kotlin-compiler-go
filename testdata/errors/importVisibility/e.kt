package e

// As kotlinc, an import through a class another file's code can't see
// is reported, not a private nested class it ends with, nor a protected
// one.
import a.WithPrivate.PrivateNested
import a.WithPrivate.PrivateObject.f // ERROR cannot access 'WithPrivate.PrivateObject': it is private in 'WithPrivate'
import a.WithPrivate.Companion.g // ERROR cannot access 'WithPrivate.Companion': it is private in 'WithPrivate'
import a.WithPrivate.PrivateNested.* // ERROR cannot access 'WithPrivate.PrivateNested': it is private in 'WithPrivate'
import a.WithPrivate.Prot.InProt
import a.HiddenOuter.Inner.h // ERROR cannot access 'HiddenOuter': it is private in file
import a.WithPrivate

fun take(a: Any) = a

fun useCompanion() {
    take(WithPrivate) // ERROR cannot access 'WithPrivate.Companion': it is private in 'WithPrivate'
    take(a.WithPrivate) // ERROR cannot access 'WithPrivate.Companion': it is private in 'WithPrivate'
}

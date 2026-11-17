public package modifiers.pkg // ERROR modifier 'public' is not applicable to 'package directive'

// kotlinc's grammar has modifiers before package: the package directive
// still names the file's package.
class A
fun f(): modifiers.pkg.A = A()
val x: Int = "" // ERROR type mismatch

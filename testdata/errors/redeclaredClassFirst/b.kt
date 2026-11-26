private class C { fun b() = 2 } // ERROR redeclaration: class C
fun fb() = C().b() // ERROR cannot access 'C': it is private in file

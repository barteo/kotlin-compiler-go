fun use() {
    hidden() // ERROR cannot access 'hidden': it is private in file
    overloaded()
    overloaded(1) // ERROR cannot access 'overloaded': it is private in file
    val c = counter // ERROR cannot access 'counter': it is private in file
    counter = 1 // ERROR cannot access 'counter': it is private in file
}

fun classes() {
    Hidden() // ERROR cannot access 'Hidden': it is private in file
    val n: Hidden.Nested? = null // ERROR cannot access 'Hidden': it is private in file
    val x = HiddenObject.x // ERROR cannot access 'HiddenObject': it is private in file
}

fun use() {
    hidden() // ERROR cannot access 'hidden': it is private in file
    overloaded()
    overloaded(1) // ERROR cannot access 'overloaded': it is private in file
    val c = counter // ERROR cannot access 'counter': it is private in file
    counter = 1 // ERROR cannot access 'counter': it is private in file
}

// MISSING_DEPENDENCY_CLASS: Java methods whose types kotlinc can't find,
// a member type Y inherits from both X and I, and a class in a file of
// another name.

fun use(y: Y) {
    y.z() // ERROR cannot access class 'Z'
    y.hidden() // ERROR cannot access class 'Hidden'
    y.ok()
    val h = Hidden() // ERROR unresolved
}

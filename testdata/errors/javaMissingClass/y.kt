// MISSING_DEPENDENCY_CLASS: Java methods whose types kotlinc can't find:
// a member type Y inherits from both X and I.

fun use(y: Y) {
    y.z() // ERROR cannot access class 'Z'
    y.ok()
}

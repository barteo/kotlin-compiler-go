// A class and a Java source class that extend each other, as kotlinc's
// test KJKComplexHierarchyNestedLoop: kotlinc reports
// CYCLIC_INHERITANCE_HIERARCHY. The cycle is broken at the Kotlin class,
// so the Java class's members aren't inherited: a call to one is
// unresolved, as kotlinc has it, and doesn't walk the cycle forever.
class K : J() { // ERROR there's a cycle in the inheritance hierarchy for this type
    fun f() {
        g() // ERROR unresolved reference: g
    }
}

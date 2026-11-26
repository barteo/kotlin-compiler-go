// Modifiers of a primary constructor need the constructor keyword after
// them, which kotlinc reports missing but parses the constructor all
// the same.

annotation class Ann(val x: Int = 1)

class A private (val x: Int) { // ERROR use the 'constructor' keyword
    inner class B @Ann(2) (val y: Int) // ERROR use the 'constructor' keyword

    fun foo() {
        class C private @Ann(3) (args: Int) // ERROR use the 'constructor' keyword
    }
}

class D @Ann(4) constructor(val z: Int)
class E @Ann private constructor(val z: Int)

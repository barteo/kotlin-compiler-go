// A class's header can't use its instance, which doesn't exist yet:
// kotlinc's INSTANCE_ACCESS_BEFORE_SUPER_CALL.

fun A.ext() = 1

class A(
    x: Int = a, // ERROR cannot access '<this>' before the instance has been initialized
    y: Int = b(), // ERROR cannot access '<this>' before the instance has been initialized
    z: Int = ext(), // ERROR cannot access '<this>' before the instance has been initialized
    i: Inner = Inner(), // ERROR before the instance has been initialized
) {
    val a = 1
    fun b() = 2
    inner class Inner
}

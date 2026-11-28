// A member whose parameter a projection of its receiver makes Nothing
// takes no argument: kotlinc's MEMBER_PROJECTED_OUT.

class Box<T> {
    fun put(t: T) {}
}

fun f(list: MutableList<out Number>, box: Box<*>) {
    list.add(42) // ERROR but Nothing was expected
    box.put("") // ERROR but Nothing was expected
}

// PROPERTY_HIDES_JAVA_FIELD: a property named as an instance field of a
// Java superclass, here AbstractList's protected modCount.

abstract class Hides : java.util.AbstractList<String>() {
    val modCount = 1 // PROPERTY_HIDES_JAVA_FIELD
}

abstract class HidesPrivately : java.util.AbstractList<String>() {
    private val modCount: Int get() = 1 // PROPERTY_HIDES_JAVA_FIELD
}

abstract class HidesInCtor(var modCount: Int) : java.util.AbstractList<String>() // PROPERTY_HIDES_JAVA_FIELD

abstract class Between : java.util.AbstractList<String>()

abstract class HidesThroughKotlin : Between() {
    open val modCount = 1 // PROPERTY_HIDES_JAVA_FIELD
}

abstract class HidesAgain : HidesThroughKotlin() {
    override val modCount = 2 // PROPERTY_HIDES_JAVA_FIELD
}

abstract class Extension : java.util.AbstractList<String>() {
    val String.modCount: Int get() = length
    val size2 = 0
}

class NotJava {
    val modCount = 0
}

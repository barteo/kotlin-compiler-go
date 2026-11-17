// A Java annotation's elements are properties to Kotlin, not functions.
fun elements(ann: Ann) {
    ann.value
    ann.count
    ann.value() // ERROR expression 'value' of type String! cannot be invoked as a function
    ann.count() // ERROR cannot be invoked as a function
    ann.hashCode()
}

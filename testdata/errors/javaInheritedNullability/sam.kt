// A Java override without nullability annotations has those of the
// method it overrides, as its SAM conversion's types.
fun main() {
    J2 { s: String -> s } // ERROR expected parameter of type String?
    J2 { "" + it.length } // ERROR only safe
    J2 { null } // ERROR type mismatch
    J2 { it ?: "" }
}

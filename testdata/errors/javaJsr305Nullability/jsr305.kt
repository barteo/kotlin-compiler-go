// JSR-305's @javax.annotation.Nullable, @CheckForNull and @Nonnull make
// Java types nullable and non-null, as kotlinc 2.4 reads them: errors,
// not warnings. @Nonnull is as its when element says: MAYBE and NEVER are
// nullable, UNKNOWN leaves a platform type. Nicknames only warn, and leave
// platform types too.

fun results(j: J) {
    j.nullable().length // ERROR only safe
    j.checkForNull().length // ERROR only safe
    j.nonnull().length
    j.always().length
    j.maybe().length // ERROR only safe
    j.never().length // ERROR only safe
    j.unknown().length
    val u: String = j.unknown()
    j.nickname().length
    val n: String = j.nickname()
}

fun parameters(j: J, s: String?) {
    j.takeNullable(null)
    j.takeNullable(s)
    j.takeNonnull(null) // ERROR type mismatch
    j.takeNonnull(s) // ERROR type mismatch
    j.takeMaybe(null)
    J(null) // ERROR type mismatch
    J(s) // ERROR type mismatch
}

fun fields(j: J) {
    j.nullableField.length // ERROR only safe
    j.nonnullField.length
    j.nonnullField = null // ERROR type mismatch
}

class Narrower : I {
    override fun get(): String = ""
    override fun name(): String = ""
    override fun put(s: String) {}
    override fun opt(s: String?) {}
}

class Wider : I { // ERROR is not abstract and does not implement abstract member
    override fun get(): String? = null
    override fun name(): String? = null // ERROR return type of 'name' is not a subtype
    override fun put(s: String?) {} // ERROR overrides nothing
    override fun opt(s: String) {} // ERROR overrides nothing
}

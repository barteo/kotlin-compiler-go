// Accessors kotlinc doesn't allow are reported each, with the rest of
// their property's errors.

val noBackingField: Int = 1 + 2 // ERROR this property has no backing field
    get() = 1
    set(value: Int) {} // ERROR a 'val'-property cannot have a setter

val defaultSetter = ""
    set // ERROR a 'val'-property cannot have a setter

var selfRead: Int = 1 + selfRead // ERROR variable 'selfRead' must be initialized
    get() = 1
    set(value) {
        field = value
    }

class Delegate {
    operator fun getValue(t: Any?, p: Any?): Int = 1
    operator fun setValue(t: Any?, p: Any?, i: Int) {}
}

var delegated: Int by Delegate()
    get() = 1 // ERROR delegated property cannot have accessors
    set(i) {} // ERROR delegated property cannot have accessors

class My {
    var x: String = ""
        set(vararg value) { // ERROR modifier 'vararg' is not applicable inside 'setter'
            field = value // ERROR type mismatch: inferred type is Array<out String>
        }
    var y: Int = 1
        set(private v) { // ERROR modifier 'private' is not applicable to 'value parameter'
            field = v
        }
}

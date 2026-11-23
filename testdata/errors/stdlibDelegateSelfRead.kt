// A delegated property read in its own delegate is reported at the
// read, as kotlinc reports it, not at the delegate.

object DelegateTest {
    var result = ""
    val f by lazy {
        result += f.toString() // ERROR type checking has run into a recursive problem; specify the type of 'f'
        "hello"
    }
    var count = 0
    val i by lazy {
        count += i // ERROR type checking has run into a recursive problem; specify the type of 'i'
        0
    }
}

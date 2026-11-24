// kotlinc's parser takes x < (... > ...) for type arguments when its
// recovering type parser ends at the >, which makes a syntax error of the
// comparison; one without a > in the parentheses is a comparison.

fun test(x: Int, y: Int): Boolean {
    if (x < (y + 1) && y > 2) return true
    if (x < (y + 1)) return x > y
    if (x < (if (y > 115) 1 else 2)) { // ERROR Type expected
        return false // ERROR 'return' is not allowed here
    }
    return true
}

// An explicit this of a lambda's receiver whose variable can't be
// inferred is reported where it is an argument, as in K2.
class TargetType
fun consumeTargetType(value: TargetType) {}
fun <PTV> buildPostponedTypeVariable(block: PTV.() -> Unit): PTV = null!!

fun test() {
    buildPostponedTypeVariable { // ERROR cannot infer type for type parameter 'PTV'
        consumeTargetType(this) // ERROR cannot infer type for type parameter 'PTV'
    }
}

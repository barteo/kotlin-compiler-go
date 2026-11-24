class InOuter<in T> {
    inner class OutIntermediate<out K> {
        inner class InInner<in G>
    }
}

fun test1(): InOuter<out Int>.OutIntermediate<String>.InInner<Char> = InOuter<Int>().OutIntermediate<String>().InInner() // ERROR projection is conflicting
fun test2(): InOuter<Int>.OutIntermediate<
    in String>.InInner<Char> = InOuter<Int>().OutIntermediate<String>().InInner() // ERROR projection is conflicting
fun test3(): InOuter<Int>.OutIntermediate<String>.InInner<
    out Char> = InOuter<Int>().OutIntermediate<String>().InInner() // ERROR projection is conflicting
fun test4(): InOuter<Int>.OutIntermediate<out String>.InInner<in Char> = InOuter<Int>().OutIntermediate<String>().InInner()

// A Java class of the file's own package hides a built-in class of its
// name, kotlin.Function here. And java.lang.Enum's describeConstable() is
// hidden, as K2 hides it: an extension of its name is called.

fun <E : Enum<E>> Enum<E>.describeConstable() = 1

enum class MyEnum { E }

fun use(f: Function<Int, String>): String {
    val g = Function { x: Int? -> x.toString() }
    val i: Int = MyEnum.E.describeConstable()
    val l: List<Int> = listOf(i)
    return f.`fun`(l[0]) + g.`fun`(2)
}

fun wrong(f: Function<Int>) {} // ERROR 2 type arguments expected

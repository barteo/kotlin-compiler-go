// Kotlin declares Int : Comparable<Int>, String : Comparable<String> and
// so on, whose arguments, unlike those of java.lang.Integer's
// Comparable<Integer>, aren't platform types: Int? isn't within a Java
// method's bound Comparable<in U!>!, while Int and a Java class's
// LocalDate? are.

import java.time.LocalDate
import java.util.Collections
import java.util.Comparator

fun nullableBool() = Comparator.comparing<String?, Boolean?> { true } // ERROR type argument is not within its bounds
fun nullableString() = Comparator.comparing<String?, String?> { "" } // ERROR type argument is not within its bounds
fun nullableInt() = Collections.max<Int?>(listOf(1)) // ERROR type argument is not within its bounds

fun nonNull() = Comparator.comparing<String?, Boolean> { true }
fun javaClass() = Comparator.comparing<String?, LocalDate?> { null }
fun inferred() = Comparator.comparing { s: String -> s.length }
fun max(xs: List<Int>): Int = Collections.max(xs)
fun sorted(xs: List<String>) = xs.sorted() + xs.sortedWith(compareBy { it.length })
fun <T : Comparable<T>> least(a: T, b: T) = if (a < b) a else b
fun leastOfThem() = least(1, 2) + least("a", "b").length

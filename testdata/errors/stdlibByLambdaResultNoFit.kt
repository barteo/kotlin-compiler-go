@file:OptIn(ExperimentalTypeInference::class)

import kotlin.experimental.ExperimentalTypeInference

fun <R> fm(f: () -> Iterable<R>): R = TODO()
@OverloadResolutionByLambdaReturnType
@JvmName("fmSeq")
fun <R> fm(f: () -> Sequence<R>): R = TODO()

fun test(elements: List<CharSequence>, b: Boolean): List<String> {
    fm { listOf(1) }
    fm { sequenceOf(1) }
    fm { // ERROR cannot infer type for type parameter 'R'
        if (b) listOf("") else null // ERROR type mismatch: inferred type is List<String>? but Iterable<
    }
    listOf(1).sumOf { "" } // ERROR ambiguous call
    return elements.flatMap {
        if (it is String) listOf("Yeah") else null // ERROR type mismatch: inferred type is List<String>? but Iterable<String> was expected
    }
}

// The warnings kotlinc 2.4.21 reports of the repeatable annotation classes below, as
// TestWarningsRedundantRepeatable reads them: each line ends with a comment naming
// the warnings it reports, if any.
import java.lang.annotation.Repeatable as JR

annotation class C1(vararg val value: A1)
@Repeatable // REDUNDANT_REPEATABLE_ANNOTATION
@java.lang.annotation.Repeatable(C1::class)
annotation class A1

annotation class C2(vararg val value: A2)
@java.lang.annotation.Repeatable(C2::class)
@kotlin.annotation.Repeatable // REDUNDANT_REPEATABLE_ANNOTATION
annotation class A2

annotation class C3(vararg val value: A3)
@JvmRepeatable(C3::class)
@Repeatable // REDUNDANT_REPEATABLE_ANNOTATION
annotation class A3

annotation class C4(vararg val value: A4)
@JR(C4::class)
@Repeatable // REDUNDANT_REPEATABLE_ANNOTATION
annotation class A4

@Repeatable
annotation class A5

annotation class C6(vararg val value: A6)
@JvmRepeatable(C6::class)
annotation class A6

annotation class C7(vararg val value: A7)
@Suppress("REDUNDANT_REPEATABLE_ANNOTATION")
@JvmRepeatable(C7::class)
@Repeatable
annotation class A7

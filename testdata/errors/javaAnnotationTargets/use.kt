@file:Targets.type

// The targets of a Java source's annotation classes: java.lang.annotation's
// TYPE is a class's or a file's, and one with both @Targets applies where
// either allows it.

@Targets.type class C

@Targets.member class D // ERROR not applicable to target 'class'

@Targets.paramOrFun fun f(@Targets.paramOrFun x: Int): Int {
    @Targets.member return x // a Java annotation on an expression isn't checked
}

@Targets.paramOrFun class E // ERROR not applicable to target 'class'

fun g(x: @Targets.kotlinType Int) = x

// With no java.lang.annotation.Target, a Kotlin one adds to Java's
// declarations, which a type parameter isn't.
@Targets.kotlinType fun h() {}

fun <@Targets.kotlinType X> k() {} // ERROR not applicable to target 'type parameter'

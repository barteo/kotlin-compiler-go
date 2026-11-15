package d

import j.JClass.foo
import b.foo

fun test() = foo // ERROR overload resolution ambiguity

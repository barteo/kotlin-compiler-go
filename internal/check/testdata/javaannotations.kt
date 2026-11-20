// The DEPRECATED_JAVA_ANNOTATION warnings below, as
// TestWarningsJavaAnnotations reads them: each line ends with a comment
// naming the warnings it reports, if any. kotlinc 2.4.21 reports them
// for these four Java annotations, not for @Repeatable or @Inherited.
@file:Suppress("UNUSED_PARAMETER")

import java.lang.annotation.ElementType
import java.lang.annotation.RetentionPolicy

@java.lang.annotation.Documented // DEPRECATED_JAVA_ANNOTATION
annotation class A1

@java.lang.annotation.Retention(RetentionPolicy.RUNTIME) // DEPRECATED_JAVA_ANNOTATION
annotation class A2

@java.lang.annotation.Target(ElementType.METHOD) // DEPRECATED_JAVA_ANNOTATION
annotation class A3

@java.lang.annotation.Inherited
annotation class A4

@java.lang.Deprecated fun g() {} // DEPRECATED_JAVA_ANNOTATION
@Deprecated("use h") fun h() {}

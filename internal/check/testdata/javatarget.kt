// The ANNOTATION_TARGETS_ONLY_IN_JAVA warnings kotlinc 2.4.21 reports
// below, as TestWarningsJavaTarget reads them: each line ends with a
// comment naming the warnings it reports, if any.
@file:Suppress("DEPRECATED_JAVA_ANNOTATION")
import java.lang.annotation.ElementType
import java.lang.annotation.Retention
import java.lang.annotation.RetentionPolicy

@java.lang.annotation.Target(ElementType.PARAMETER) // ANNOTATION_TARGETS_ONLY_IN_JAVA
annotation class D

@Target(AnnotationTarget.CLASS)
@java.lang.annotation.Target(ElementType.TYPE)
annotation class E

@kotlin.annotation.Target(AnnotationTarget.CLASS)
@java.lang.annotation.Target(ElementType.TYPE)
annotation class F

@Retention(RetentionPolicy.RUNTIME)
@java.lang.annotation.Target(ElementType.ANNOTATION_TYPE, ElementType.CONSTRUCTOR) // ANNOTATION_TARGETS_ONLY_IN_JAVA
annotation class G

@Retention(RetentionPolicy.RUNTIME)
annotation class H

class Outer {
    @java.lang.annotation.Target(ElementType.METHOD) // ANNOTATION_TARGETS_ONLY_IN_JAVA
    annotation class Nested
}

// kotlinc reads some annotations before it resolves types, so a
// typealias can't stand for them
// (TYPEALIAS_EXPANDS_TO_COMPILER_REQUIRED_ANNOTATION_ERROR).
typealias DeprecatedTA = kotlin.Deprecated // ERROR typealias to a compiler-required annotation 'kotlin.Deprecated'
typealias JvmNameTA = kotlin.jvm.JvmName // ERROR typealias to a compiler-required annotation 'kotlin.jvm.JvmName'
typealias JavaDeprecatedTA = java.lang.Deprecated // ERROR typealias to a compiler-required annotation 'java.lang.Deprecated'
typealias JvmInlineTA = kotlin.jvm.JvmInline // ERROR typealias to a compiler-required annotation 'kotlin.jvm.JvmInline'
typealias JvmStaticTA = kotlin.jvm.JvmStatic
typealias ThrowsTA = kotlin.jvm.Throws

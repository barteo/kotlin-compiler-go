// Replaces the ring suite's adapter class of the same name
// (kotlin-native/performance/benchmarksKotlinxAdapter), which declares
// baseOnly as a JMH @Param. JMH accepts @Param only in @State classes, and
// baseOnly is always false in this run, so this one has no parameter.
package org.jetbrains.benchmarksLauncher

abstract class SkipWhenBaseOnly {
    fun skipWhenBaseOnly() {}
}

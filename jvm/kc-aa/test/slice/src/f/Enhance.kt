package f

// Java methods as FIR enhances them from the Kotlin members they
// override, and intersection overrides (docs/analysis-api.md, step 30).

/** AbstractMap's get/remove return V?, equals takes Any?, containsKey(Object) is containsKey(K). */
abstract class Table : java.util.AbstractMap<String, Int>()

/** contains(Object) and remove(Object) are contains(E) and remove(E); toString is AbstractSet's intersection override. */
abstract class Names : java.util.AbstractSet<String>()

interface Runs { fun run() }
open class Runner : Runs { override fun run() {} }

/** Runner.run overrides Runs.run, and Both still has an intersection override of the two. */
class Both : Runner(), Runs

// The typealias imported under another name.
package renamed

import kotlin.collections.HashSet as KSet

fun takeSet(s: Set<String>) {}

fun rejected() {
    takeSet(KSet(setOf(null))) // ERROR type mismatch
}

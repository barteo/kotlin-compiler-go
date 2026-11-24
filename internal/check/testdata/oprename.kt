// The warnings kotlinc 2.4.21 reports of the imports below, as
// TestWarningsOperatorRenamed reads them: each line ends with a comment naming
// the warnings it reports, if any.
package p

import p.plus as minus // OPERATOR_RENAMED_ON_IMPORT
import p.plus as foo
import p.plus as mod
import p.plus as component3 // OPERATOR_RENAMED_ON_IMPORT
import p.plus as of // OPERATOR_RENAMED_ON_IMPORT
import p.notOp as times
import p.Obj.times as div // OPERATOR_RENAMED_ON_IMPORT
import p.Obj.plain as rem
import p.Cls.Companion.inc as dec // OPERATOR_RENAMED_ON_IMPORT
import kotlin.collections.plus as get // OPERATOR_RENAMED_ON_IMPORT
import kotlin.collections.plus as add
import kotlin.text.get as invoke // OPERATOR_RENAMED_ON_IMPORT
import kotlin.collections.component1 as component2 // OPERATOR_RENAMED_ON_IMPORT
import kotlin.collections.getOrPut as set
import kotlin.math.max as plus

interface A
operator fun A.plus(o: A): A = this
fun A.notOp(o: A): A = this
object Obj {
    operator fun A.times(o: A): A = this
    fun A.plain(o: A): A = this
}
class Cls {
    companion object {
        operator fun A.inc(): A = this
    }
}

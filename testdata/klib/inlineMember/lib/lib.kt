// A library object's public inline member, as kotlinx-io's
// UnsafeBufferOperations.readFromHead: its body is copied, so the lambda
// passed to it may suspend the caller and assign the caller's locals.
package lib

import kotlin.contracts.ExperimentalContracts
import kotlin.contracts.InvocationKind
import kotlin.contracts.contract

class Buf(val bytes: ByteArray) {
    var pos = 0
}

object Ops {
    @OptIn(ExperimentalContracts::class)
    inline fun readFromHead(buf: Buf, action: (ByteArray, Int, Int) -> Int): Int {
        contract { callsInPlace(action, InvocationKind.EXACTLY_ONCE) }
        val n = action(buf.bytes, buf.pos, buf.bytes.size)
        if (n < 0) throw IllegalStateException("negative")
        if (n > buf.bytes.size - buf.pos) throw IllegalStateException("too many")
        buf.pos += n
        return n
    }
}

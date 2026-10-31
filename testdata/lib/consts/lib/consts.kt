package consts

import kotlin.math.E as EULER

// Consts whose values are Java constants, and the standard library's
// compiled consts: their fields have ConstantValues, which kotlinc reads
// when it compiles code that uses them.
const val MAX = Integer.MAX_VALUE
const val DECEMBER = java.util.Calendar.DECEMBER + 1
const val RADIX = "radix " + Character.MAX_RADIX
const val BYTE = java.lang.Byte.MAX_VALUE
const val BIG = java.lang.Long.MIN_VALUE
const val PI = Math.PI
const val BUFFER = DEFAULT_BUFFER_SIZE * 2
const val E = EULER

object Codes {
    const val FIRST_USER = java.net.HttpURLConnection.HTTP_OK + 1
}

class Holder {
    companion object {
        const val SHIFTED = java.awt.event.KeyEvent.VK_A shl 1
    }
}

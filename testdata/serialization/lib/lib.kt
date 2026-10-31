package lib

import kotlinx.serialization.KSerializer
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.descriptors.SerialDescriptor
import kotlinx.serialization.encoding.AbstractDecoder
import kotlinx.serialization.encoding.AbstractEncoder
import kotlinx.serialization.encoding.CompositeDecoder
import kotlinx.serialization.encoding.CompositeEncoder
import kotlinx.serialization.modules.EmptySerializersModule
import kotlinx.serialization.modules.SerializersModule

// Serializable classes of another module, compiled by kotlinc, which the
// program's classes have properties of.
@Serializable
data class LibPoint(val x: Int, val y: Int = 0)

@Serializable
enum class LibColor { RED, @SerialName("verde") GREEN }

enum class LibPlain { A, B }

@Serializable
sealed class LibShape {
    @Serializable
    @SerialName("dot")
    data object Dot : LibShape()

    @Serializable
    data class Sq(val side: Int) : LibShape()
}

@Serializable
data object LibObj

// A superclass from another module, which kc leaves to kotlinc.
@Serializable
open class LibBase(val b: Int = 0)

// A format that writes values to a list, and reads them back in order:
// unlike JSON's, its decoders decode sequentially.
class ListEncoder : AbstractEncoder() {
    val list = mutableListOf<Any?>()
    override val serializersModule: SerializersModule = EmptySerializersModule()
    override fun encodeValue(value: Any) {
        list.add(value)
    }

    override fun encodeNull() {
        list.add(null)
    }

    override fun encodeNotNullMark() {
        list.add("!")
    }

    override fun beginCollection(descriptor: SerialDescriptor, collectionSize: Int): CompositeEncoder {
        encodeInt(collectionSize)
        return this
    }
}

class ListDecoder(private val list: ArrayDeque<Any?>, private var count: Int = 0) : AbstractDecoder() {
    private var index = 0
    override val serializersModule: SerializersModule = EmptySerializersModule()
    override fun decodeValue(): Any = list.removeFirst()!!
    override fun decodeElementIndex(descriptor: SerialDescriptor): Int =
        if (index == count) CompositeDecoder.DECODE_DONE else index++

    override fun beginStructure(descriptor: SerialDescriptor): CompositeDecoder = ListDecoder(list, descriptor.elementsCount)
    override fun decodeSequentially(): Boolean = true
    override fun decodeCollectionSize(descriptor: SerialDescriptor): Int = decodeInt().also { count = it }
    override fun decodeNotNullMark(): Boolean = list.removeFirst() != null
}

// listRoundTrip writes v to a list, and reads it back.
fun <T> listRoundTrip(s: KSerializer<T>, v: T): String {
    val e = ListEncoder()
    e.encodeSerializableValue(s, v)
    val back = ListDecoder(ArrayDeque(e.list)).decodeSerializableValue(s)
    return "${e.list} -> $back"
}

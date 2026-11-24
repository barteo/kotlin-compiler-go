package app

import app.Wrappers.ListWrapSer
import kotlinx.serialization.KSerializer
import kotlinx.serialization.Serializable
import kotlinx.serialization.builtins.ListSerializer
import kotlinx.serialization.builtins.serializer
import kotlinx.serialization.descriptors.PrimitiveKind
import kotlinx.serialization.descriptors.PrimitiveSerialDescriptor
import kotlinx.serialization.descriptors.SerialDescriptor
import kotlinx.serialization.encoding.Decoder
import kotlinx.serialization.encoding.Encoder

// Typealiases whose types name their serializers with @Serializable(with),
// which the plugin uses where a property's type names the alias. The
// serializer of a generic type takes its type arguments' serializers.
interface Crate<T> {
    val v: T
}

data class CrateImpl<T>(override val v: T) : Crate<T>

class CrateSer<T>(private val s: KSerializer<T>) : KSerializer<Crate<T>> {
    override val descriptor: SerialDescriptor = s.descriptor
    override fun serialize(encoder: Encoder, value: Crate<T>) = s.serialize(encoder, value.v)
    override fun deserialize(decoder: Decoder): Crate<T> = CrateImpl(s.deserialize(decoder))
}

typealias SCrate<T> = @Serializable(with = CrateSer::class) Crate<T>

// An alias of an alias.
typealias IntCrate = SCrate<Int>

data class ListWrap<T>(val items: List<T>)

// The serializer is found where the alias is declared: this file imports
// it, the file that uses the alias doesn't.
object Wrappers {
    class ListWrapSer<T>(s: KSerializer<T>) : KSerializer<ListWrap<T>> {
        private val list = ListSerializer(s)
        override val descriptor: SerialDescriptor = list.descriptor
        override fun serialize(encoder: Encoder, value: ListWrap<T>) = list.serialize(encoder, value.items)
        override fun deserialize(decoder: Decoder) = ListWrap(list.deserialize(decoder))
    }
}

typealias SerializableListWrap<T> = @Serializable(ListWrapSer::class) ListWrap<T>

object ShoutSer : KSerializer<String> {
    override val descriptor = PrimitiveSerialDescriptor("Shout", PrimitiveKind.STRING)
    override fun serialize(encoder: Encoder, value: String) = encoder.encodeString(value.uppercase())
    override fun deserialize(decoder: Decoder) = decoder.decodeString().lowercase()
}

typealias Shouted = @Serializable(with = ShoutSer::class) String

fun intSer() = Int.serializer()

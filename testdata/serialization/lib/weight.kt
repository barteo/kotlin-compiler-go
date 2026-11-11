package lib

import kotlinx.serialization.KSerializer
import kotlinx.serialization.descriptors.PrimitiveKind
import kotlinx.serialization.descriptors.PrimitiveSerialDescriptor
import kotlinx.serialization.descriptors.SerialDescriptor
import kotlinx.serialization.encoding.Decoder
import kotlinx.serialization.encoding.Encoder

object WeightSerializer : KSerializer<Weight> {
    override val descriptor: SerialDescriptor = PrimitiveSerialDescriptor("lib.Weight", PrimitiveKind.STRING)
    override fun serialize(encoder: Encoder, value: Weight) = encoder.encodeString("${value.grams}g")
    override fun deserialize(decoder: Decoder): Weight = Weight(decoder.decodeString().removeSuffix("g").toInt())
}

package app

import kotlinx.serialization.KSerializer
import kotlinx.serialization.Serializable
import kotlinx.serialization.descriptors.PrimitiveKind
import kotlinx.serialization.descriptors.PrimitiveSerialDescriptor
import kotlinx.serialization.descriptors.SerialDescriptor
import kotlinx.serialization.encoding.Decoder
import kotlinx.serialization.encoding.Encoder
import lib.Weight

class StickerSerializer : KSerializer<Sticker> {
    override val descriptor: SerialDescriptor = PrimitiveSerialDescriptor("app.Sticker", PrimitiveKind.STRING)
    override fun serialize(encoder: Encoder, value: Sticker) = encoder.encodeString("[${value.text}]")
    override fun deserialize(decoder: Decoder): Sticker = Sticker(decoder.decodeString().removeSurrounding("[", "]"))
}

// Properties of Java classes annotated @Serializable(with = ...).
@Serializable
data class Parcel(val sticker: Sticker, val weight: Weight, val extra: Weight? = null)

class TagSerializer : KSerializer<Tag> {
    override val descriptor: SerialDescriptor = PrimitiveSerialDescriptor("app.Tag", PrimitiveKind.STRING)
    override fun serialize(encoder: Encoder, value: Tag) = encoder.encodeString("<${value.text}>")
    override fun deserialize(decoder: Decoder): Tag = Tag(decoder.decodeString().removeSurrounding("<", ">"))
}

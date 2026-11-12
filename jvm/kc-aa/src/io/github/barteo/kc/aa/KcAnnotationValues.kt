package io.github.barteo.kc.aa

import org.jetbrains.kotlin.analysis.api.annotations.KaAnnotation
import org.jetbrains.kotlin.analysis.api.annotations.KaAnnotationValue
import org.jetbrains.kotlin.analysis.api.annotations.KaNamedAnnotationValue
import org.jetbrains.kotlin.analysis.api.base.KaConstantValue
import org.jetbrains.kotlin.analysis.api.lifetime.KaLifetimeToken
import org.jetbrains.kotlin.analysis.api.types.KaType
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.psi.KtElement

/** An annotation's argument: its parameter's [name] and its value. */
internal class KcNamedValue(
    override val name: Name,
    override val expression: KaAnnotationValue,
    override val token: KaLifetimeToken,
) : KaNamedAnnotationValue {
    override fun toString() = "$name = $expression"
}

internal class KcConstantAnnotationValue(
    override val value: KaConstantValue,
    override val token: KaLifetimeToken,
) : KaAnnotationValue.ConstantValue {
    override val sourcePsi get() = value.sourcePsi
    override fun toString() = value.render()
}

internal class KcEnumEntryValue(
    override val callableId: CallableId?,
    override val sourcePsi: KtElement?,
    override val token: KaLifetimeToken,
) : KaAnnotationValue.EnumEntryValue {
    override fun toString() = callableId.toString()
}

internal class KcClassLiteralValue(
    override val type: KaType,
    override val classId: ClassId?,
    override val sourcePsi: KtElement?,
    override val token: KaLifetimeToken,
) : KaAnnotationValue.ClassLiteralValue {
    override fun toString() = "$classId::class"
}

internal class KcNestedAnnotationValue(
    override val annotation: KaAnnotation,
    override val sourcePsi: KtElement?,
    override val token: KaLifetimeToken,
) : KaAnnotationValue.NestedAnnotationValue {
    override fun toString() = annotation.toString()
}

internal class KcArrayValue(
    override val values: Collection<KaAnnotationValue>,
    override val sourcePsi: KtElement?,
    override val token: KaLifetimeToken,
) : KaAnnotationValue.ArrayValue {
    override fun toString() = values.joinToString(", ", "[", "]")
}

internal class KcUnsupportedValue(
    override val sourcePsi: KtElement?,
    override val token: KaLifetimeToken,
) : KaAnnotationValue.UnsupportedValue {
    override fun toString() = "<unsupported>"
}

/**
 * The constants, as namespaces of KaConstantValue's subinterfaces (it is
 * sealed); render() writes them as Kotlin does.
 */
internal object KcConstant {
    class Null(override val sourcePsi: KtElement?) : KaConstantValue.NullValue {
        override val value: Any? get() = null
        override fun render() = "null"
        override fun toString() = render()
    }
    class Boolean(override val value: kotlin.Boolean, override val sourcePsi: KtElement?) : KaConstantValue.BooleanValue {
        override fun render() = value.toString()
        override fun toString() = render()
    }
    class Char(override val value: kotlin.Char, override val sourcePsi: KtElement?) : KaConstantValue.CharValue {
        override fun render() = "'$value'"
        override fun toString() = render()
    }
    class Byte(override val value: kotlin.Byte, override val sourcePsi: KtElement?) : KaConstantValue.ByteValue {
        override fun render() = value.toString()
        override fun toString() = render()
    }
    class UByte(override val value: kotlin.UByte, override val sourcePsi: KtElement?) : KaConstantValue.UByteValue {
        override fun render() = "${value}u"
        override fun toString() = render()
    }
    class Short(override val value: kotlin.Short, override val sourcePsi: KtElement?) : KaConstantValue.ShortValue {
        override fun render() = value.toString()
        override fun toString() = render()
    }
    class UShort(override val value: kotlin.UShort, override val sourcePsi: KtElement?) : KaConstantValue.UShortValue {
        override fun render() = "${value}u"
        override fun toString() = render()
    }
    class Int(override val value: kotlin.Int, override val sourcePsi: KtElement?) : KaConstantValue.IntValue {
        override fun render() = value.toString()
        override fun toString() = render()
    }
    class UInt(override val value: kotlin.UInt, override val sourcePsi: KtElement?) : KaConstantValue.UIntValue {
        override fun render() = "${value}u"
        override fun toString() = render()
    }
    class Long(override val value: kotlin.Long, override val sourcePsi: KtElement?) : KaConstantValue.LongValue {
        override fun render() = "${value}L"
        override fun toString() = render()
    }
    class ULong(override val value: kotlin.ULong, override val sourcePsi: KtElement?) : KaConstantValue.ULongValue {
        override fun render() = "${value}uL"
        override fun toString() = render()
    }
    class Float(override val value: kotlin.Float, override val sourcePsi: KtElement?) : KaConstantValue.FloatValue {
        override fun render() = "${value}f"
        override fun toString() = render()
    }
    class Double(override val value: kotlin.Double, override val sourcePsi: KtElement?) : KaConstantValue.DoubleValue {
        override fun render() = value.toString()
        override fun toString() = render()
    }
    class String(override val value: kotlin.String, override val sourcePsi: KtElement?) : KaConstantValue.StringValue {
        override fun render() = "\"$value\""
        override fun toString() = render()
    }

    fun of(c: KcValue.Const, psi: KtElement?): KaConstantValue = when (c.kind) {
        KcValue.Const.NULL -> Null(psi)
        KcValue.Const.BOOLEAN -> Boolean(c.bits != 0L, psi)
        KcValue.Const.CHAR -> Char(c.bits.toInt().toChar(), psi)
        KcValue.Const.BYTE -> Byte(c.bits.toByte(), psi)
        KcValue.Const.UBYTE -> UByte(c.bits.toUByte(), psi)
        KcValue.Const.SHORT -> Short(c.bits.toShort(), psi)
        KcValue.Const.USHORT -> UShort(c.bits.toUShort(), psi)
        KcValue.Const.INT -> Int(c.bits.toInt(), psi)
        KcValue.Const.UINT -> UInt(c.bits.toUInt(), psi)
        KcValue.Const.LONG -> Long(c.bits, psi)
        KcValue.Const.ULONG -> ULong(c.bits.toULong(), psi)
        KcValue.Const.FLOAT -> Float(c.float.toFloat(), psi)
        KcValue.Const.DOUBLE -> Double(c.float, psi)
        KcValue.Const.STRING -> String(c.str, psi)
        else -> error("kc: constant kind ${c.kind}")
    }
}

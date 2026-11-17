@file:Suppress("DEPRECATION", "DEPRECATION_ERROR", "OVERRIDE_DEPRECATION")

package io.github.barteo.kc.aa

import org.jetbrains.kotlin.analysis.api.resolution.*
import org.jetbrains.kotlin.analysis.api.signatures.KaFunctionSignature
import org.jetbrains.kotlin.analysis.api.signatures.KaVariableSignature
import org.jetbrains.kotlin.analysis.api.symbols.*
import org.jetbrains.kotlin.analysis.api.types.KaSubstitutor
import org.jetbrains.kotlin.analysis.api.types.KaType
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.psi.KtExpression

internal class KcLintFunctionSignature<S : KaFunctionSymbol>(
    override val symbol: S,
    override val returnType: KaType,
    override val receiverType: KaType?,
    override val valueParameters: List<KaVariableSignature<KaValueParameterSymbol>>,
) : StubKaFunctionSignature<S>() {
    override val token get() = symbol.token
    override fun substitute(substitutor: KaSubstitutor): KaFunctionSignature<S> = this
    override fun equals(other: Any?) = other is KcLintFunctionSignature<*> && other.symbol == symbol && other.returnType == returnType
    override fun hashCode() = symbol.hashCode()
}

internal class KcLintVariableSignature<S : KaVariableSymbol>(
    override val symbol: S,
    override val name: Name,
    override val returnType: KaType,
    override val receiverType: KaType? = null,
) : StubKaVariableSignature<S>() {
    override val token get() = symbol.token
    override fun substitute(substitutor: KaSubstitutor): KaVariableSignature<S> = this
    override fun equals(other: Any?) = other is KcLintVariableSignature<*> && other.symbol == symbol && other.returnType == returnType
    override fun hashCode() = symbol.hashCode()
}

internal class KcLintPartiallyApplied<S : KaCallableSymbol, C : org.jetbrains.kotlin.analysis.api.signatures.KaCallableSignature<S>>(
    override val signature: C,
    override val dispatchReceiver: KaReceiverValue?,
    override val extensionReceiver: KaReceiverValue?,
) : StubKaPartiallyAppliedSymbol<S, C>() {
    override val token get() = signature.token
    override val contextArguments: List<KaReceiverValue> get() = emptyList()
}

internal class KcLintExplicitReceiver(override val expression: KtExpression, override val isSafeNavigation: Boolean, override val type: KaType) :
    StubKaExplicitReceiverValue() {
    override val token get() = type.token
}

internal class KcLintImplicitReceiver(override val symbol: KaSymbol, override val type: KaType) : StubKaImplicitReceiverValue() {
    override val token get() = type.token
}

internal class KcLintSmartCastReceiver(override val original: KaReceiverValue, override val type: KaType) : StubKaSmartCastedReceiverValue() {
    override val token get() = type.token
}

@Suppress("UNCHECKED_CAST")
internal class KcLintSimpleFunctionCall(
    override val partiallyAppliedSymbol: KaPartiallyAppliedSymbol<KaFunctionSymbol, KaFunctionSignature<KaFunctionSymbol>>,
    override val valueArgumentMapping: Map<KtExpression, KaVariableSignature<KaValueParameterSymbol>>,
    override val isImplicitInvoke: Boolean,
) : StubKaSimpleFunctionCall() {
    override val token get() = partiallyAppliedSymbol.token
    override val signature get() = partiallyAppliedSymbol.signature
    override val dispatchReceiver get() = partiallyAppliedSymbol.dispatchReceiver
    override val extensionReceiver get() = partiallyAppliedSymbol.extensionReceiver
    override val contextArguments: List<KaReceiverValue> get() = emptyList()
    override val typeArgumentsMapping: Map<KaTypeParameterSymbol, KaType> get() = emptyMap()
    override val contextArgumentMapping: Map<KtExpression, KaVariableSignature<KaContextParameterSymbol>> get() = emptyMap()
    override val combinedArgumentMapping: Map<KtExpression, KaVariableSignature<KaParameterSymbol>>
        get() = valueArgumentMapping as Map<KtExpression, KaVariableSignature<KaParameterSymbol>>
    override fun toString() = "call ${signature.symbol}"
}

internal object KcLintRead : KaVariableAccessCall.Kind.Read, KaSimpleVariableAccess.Read

internal class KcLintWrite(override val value: KtExpression?) : KaVariableAccessCall.Kind.Write, KaSimpleVariableAccess.Write

internal class KcLintVariableAccessCall(
    override val partiallyAppliedSymbol: KaPartiallyAppliedSymbol<KaVariableSymbol, KaVariableSignature<KaVariableSymbol>>,
    override val kind: KaVariableAccessCall.Kind,
) : StubKaSimpleVariableAccessCall() {
    override val token get() = partiallyAppliedSymbol.token
    override val signature get() = partiallyAppliedSymbol.signature
    override val dispatchReceiver get() = partiallyAppliedSymbol.dispatchReceiver
    override val extensionReceiver get() = partiallyAppliedSymbol.extensionReceiver
    override val contextArguments: List<KaReceiverValue> get() = emptyList()
    override val typeArgumentsMapping: Map<KaTypeParameterSymbol, KaType> get() = emptyMap()
    override val isContextSensitive: Boolean get() = false
    override val simpleAccess: KaSimpleVariableAccess get() = kind as KaSimpleVariableAccess
    override fun toString() = "access ${signature.symbol}"
}

internal class KcLintSuccessCallInfo(override val call: KaCall) : StubKaSuccessCallInfo() {
    override val token get() = call.token
}

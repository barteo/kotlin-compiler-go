@file:Suppress("DEPRECATION", "OVERRIDE_DEPRECATION")

package io.github.barteo.kc.aa

import com.intellij.psi.PsiElement
import org.jetbrains.kotlin.analysis.api.KaSession
import org.jetbrains.kotlin.analysis.api.annotations.KaAnnotation
import org.jetbrains.kotlin.analysis.api.annotations.KaAnnotationList
import org.jetbrains.kotlin.analysis.api.annotations.KaNamedAnnotationValue
import org.jetbrains.kotlin.analysis.api.base.KaContextReceiver
import org.jetbrains.kotlin.analysis.api.lifetime.KaLifetimeToken
import org.jetbrains.kotlin.analysis.api.scopes.KaScope
import org.jetbrains.kotlin.analysis.api.symbols.*
import org.jetbrains.kotlin.analysis.api.symbols.markers.KaNamedSymbol
import org.jetbrains.kotlin.analysis.api.symbols.pointers.KaSymbolPointer
import org.jetbrains.kotlin.analysis.api.types.KaType
import org.jetbrains.kotlin.analysis.api.types.KaTypeProjection
import org.jetbrains.kotlin.analysis.api.types.KaResolvedClassTypeQualifier
import org.jetbrains.kotlin.descriptors.Visibilities
import org.jetbrains.kotlin.descriptors.Visibility
import org.jetbrains.kotlin.descriptors.annotations.AnnotationUseSiteTarget
import org.jetbrains.kotlin.descriptors.java.JavaVisibilities
import org.jetbrains.kotlin.analysis.api.annotations.KaAnnotationValue
import org.jetbrains.kotlin.lexer.KtModifierKeywordToken
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.name.SpecialNames
import org.jetbrains.kotlin.psi.*
import org.jetbrains.kotlin.types.Variance

/** The lifetime of one round's symbols: valid until the next update replaces them. */
internal class KcToken : KaLifetimeToken() {
    @Volatile
    var invalidation: String? = null

    override fun isValid() = invalidation == null
    override fun getInvalidationReason() = invalidation ?: "valid"
    override fun isAccessible() = true
    override fun getInaccessibilityReason() = "accessible"
}

/**
 * What every symbol of a round's declarations knows: its declaration in the
 * Go model, its PSI (null for a library symbol), the symbol it is declared
 * in (null for a top-level library class) and its pointer key.
 */
internal class KcInfo(
    val round: KcRound,
    val decl: KcDecl,
    val psi: PsiElement?,
    val parent: KaSymbol?,
    val key: String,
    /** The annotations of the declaration that apply to this symbol. */
    annotations: List<KcAnnotation>,
    /** Where the declaration's types are read. */
    val types: KcTypeContext,
    /** SOURCE, LIBRARY, JAVA_LIBRARY or JAVA_SOURCE. */
    val origin: KaSymbolOrigin,
) {
    /** The KaType of [t], or [orElse] if it is null. */
    fun type(t: KcType?, orElse: () -> KaType = { KcErrorType(round, "") }): KaType = t?.let(types::type) ?: orElse()

    val unit: KaType get() = KcClassType(round, ClassId.fromString("kotlin/Unit"))

    /** The receiver parameter of [owner], an extension callable, or null if it isn't one. */
    fun receiver(owner: KaCallableSymbol): KaReceiverParameterSymbol? =
        decl.receiver?.let { KcReceiverParameterSymbol(this, owner, type(it)) }

    val annotations: KaAnnotationList by lazy { KcAnnotationList(annotations.map { round.annotation(it) }, round.token) }

    val name: Name get() = Name.identifier(decl.name)

    val visibility: KaSymbolVisibility
        get() = when (decl.visibility) {
            KcDecl.PRIVATE -> KaSymbolVisibility.PRIVATE
            KcDecl.PROTECTED -> KaSymbolVisibility.PROTECTED
            KcDecl.INTERNAL -> KaSymbolVisibility.INTERNAL
            KcDecl.PACKAGE_PRIVATE -> KaSymbolVisibility.PACKAGE_PRIVATE
            else -> KaSymbolVisibility.PUBLIC
        }

    val compilerVisibility: Visibility
        get() = when (decl.visibility) {
            KcDecl.PRIVATE -> Visibilities.Private
            KcDecl.PROTECTED -> if (isJava) JavaVisibilities.ProtectedAndPackage else Visibilities.Protected
            KcDecl.INTERNAL -> Visibilities.Internal
            KcDecl.PACKAGE_PRIVATE -> JavaVisibilities.PackageVisibility
            else -> Visibilities.Public
        }

    /** Whether the declaration is a Java class's. */
    val isJava get() = origin == KaSymbolOrigin.JAVA_LIBRARY || origin == KaSymbolOrigin.JAVA_SOURCE

    val location: KaSymbolLocation
        get() = when (parent) {
            is KaClassSymbol -> KaSymbolLocation.CLASS
            is KaFileSymbol, is KaPackageSymbol, null -> KaSymbolLocation.TOP_LEVEL
            else -> KaSymbolLocation.LOCAL
        }

    fun has(token: KtModifierKeywordToken) = (psi as? KtModifierListOwner)?.hasModifier(token) == true

    val modality: KaSymbolModality
        get() = when (decl.modality) {
            KcDecl.SEALED -> KaSymbolModality.SEALED
            KcDecl.ABSTRACT -> KaSymbolModality.ABSTRACT
            KcDecl.OPEN -> KaSymbolModality.OPEN
            else -> KaSymbolModality.FINAL
        }

    /** The callable ID of a member or top-level callable named [name]. */
    fun callableId(name: Name): CallableId? = when (parent) {
        is KcClassSymbol -> parent.classId?.let { CallableId(it, name) }
        is KcFileSymbol -> CallableId(FqName(parent.file.packageName), name)
        is KcPackageSymbol -> CallableId(parent.fqName, name)
        else -> null
    }

    fun <S : KaSymbol> pointer(): KaSymbolPointer<S> = KcPointer(key)
}

/** A pointer that restores a symbol by its key in the current round. */
internal class KcPointer<S : KaSymbol>(private val key: String) : StubKaSymbolPointer<S>() {
    @Suppress("UNCHECKED_CAST")
    override fun restoreSymbol(analysisSession: KaSession): S? = KcAnalysis.of(analysisSession).round.restore(key) as S?
}

/** A package, of the sources or of the class path; library top-level declarations' parent. */
internal class KcPackageSymbol(val round: KcRound, override val fqName: FqName) : StubKaPackageSymbol() {
    override val token get() = round.token
    override val psi: PsiElement? get() = null
    override val origin get() = KaSymbolOrigin.SOURCE
    override fun createPointer(): KaSymbolPointer<KaPackageSymbol> = KcPointer("pkgsym:" + fqName.asString())
    override fun equals(other: Any?) = other is KcPackageSymbol && other.fqName == fqName
    override fun hashCode() = fqName.hashCode()
    override fun toString() = "KcPackageSymbol($fqName)"
}

internal class KcFileSymbol(val round: KcRound, val file: KcFile, override val psi: KtFile) : StubKaFileSymbol() {
    override val token get() = round.token
    override val origin get() = KaSymbolOrigin.SOURCE
    override val annotations: KaAnnotationList by lazy {
        KcAnnotationList(file.annotations.map { round.annotation(it) }, round.token)
    }
    val key = "file:" + file.path
    override fun createPointer(): KaSymbolPointer<KaFileSymbol> = KcPointer(key)

    lateinit var declarations: List<KaDeclarationSymbol>
}

internal class KcClassSymbol(val info: KcInfo) : StubKaNamedClassSymbol() {
    override val token get() = info.round.token
    override val psi get() = info.psi
    override val origin get() = info.origin
    override val annotations get() = info.annotations
    override val name get() = info.name
    override val classId: ClassId? = ClassId.fromString(info.decl.classId)
    override val visibility get() = info.visibility
    override val compilerVisibility get() = info.compilerVisibility
    override val location get() = info.location
    override val modality get() = info.modality
    override val superTypes: List<KaType> by lazy { info.decl.supertypes.map { info.type(it) } }
    /** The type of this class with its own type parameters as arguments, C<T>. */
    val selfType: KaType
        get() = KcClassType(info.round, classId ?: ClassId.fromString("kotlin/Any"), typeParams.map {
            KcTypeArgument(KcTypeParameterType(info.round, it.name, it, false), Variance.INVARIANT, info.round.token)
        })
    override val classKind: KaClassKind
        get() = when (info.decl.kind) {
            KcDecl.INTERFACE -> KaClassKind.INTERFACE
            KcDecl.OBJECT -> if (info.decl.isCompanion) KaClassKind.COMPANION_OBJECT else KaClassKind.OBJECT
            KcDecl.ENUM_CLASS -> KaClassKind.ENUM_CLASS
            KcDecl.ANNOTATION_CLASS -> KaClassKind.ANNOTATION_CLASS
            else -> KaClassKind.CLASS
        }
    override val isData get() = info.decl.isData
    override val isInner get() = info.decl.has(KcDecl.INNER)
    override val isInline get() = info.decl.has(KcDecl.VALUE)
    override val isFun get() = info.decl.has(KcDecl.FUN)
    override val isExternal get() = info.decl.has(KcDecl.EXTERNAL)
    override val isExpect get() = info.decl.has(KcDecl.EXPECT)
    override val isActual get() = info.decl.has(KcDecl.ACTUAL)
    override val contextReceivers: List<KaContextReceiver> get() = emptyList()
    override fun createPointer(): KaSymbolPointer<KaNamedClassSymbol> = info.pointer()

    lateinit var typeParams: List<KaTypeParameterSymbol>
    override val typeParameters get() = typeParams

    /** The members: callables, constructors, enum entries, nested classes. */
    lateinit var members: List<KaDeclarationSymbol>
    override val companionObject: KaNamedClassSymbol?
        get() = members.firstOrNull { it is KcClassSymbol && it.info.decl.isCompanion } as KaNamedClassSymbol?
}

internal class KcFunctionSymbol(val info: KcInfo) : StubKaNamedFunctionSymbol() {
    override val token get() = info.round.token
    override val psi get() = info.psi
    override val origin get() = info.origin
    override val annotations get() = info.annotations
    override val name get() = info.name
    override val callableId get() = info.callableId(name)
    override val visibility get() = info.visibility
    override val compilerVisibility get() = info.compilerVisibility
    override val location get() = info.location
    override val modality get() = info.modality
    override val returnType: KaType by lazy { info.type(info.decl.type) { info.unit } }
    override val receiverParameter: KaReceiverParameterSymbol? by lazy { info.receiver(this) }
    override val isSuspend get() = info.decl.has(KcDecl.SUSPEND)
    override val isInline get() = info.decl.has(KcDecl.INLINE)
    override val isOperator get() = info.decl.has(KcDecl.OPERATOR)
    override val isInfix get() = info.decl.has(KcDecl.INFIX)
    override val isTailRec get() = info.decl.has(KcDecl.TAILREC)
    override val isExternal get() = info.decl.has(KcDecl.EXTERNAL)
    override val isOverride get() = info.decl.has(KcDecl.OVERRIDE)
    override val isExpect get() = info.decl.has(KcDecl.EXPECT)
    override val isActual get() = info.decl.has(KcDecl.ACTUAL)
    override val isExtension get() = info.decl.receiver != null
    override val isStatic get() = info.decl.has(KcDecl.STATIC)
    override val isBuiltinFunctionInvoke get() = false
    override val hasStableParameterNames get() = !info.isJava
    override val contextReceivers: List<KaContextReceiver> get() = emptyList()
    override val contextParameters: List<KaContextParameterSymbol> get() = emptyList()
    override fun createPointer(): KaSymbolPointer<KaNamedFunctionSymbol> = info.pointer()

    lateinit var typeParams: List<KaTypeParameterSymbol>
    lateinit var params: List<KaValueParameterSymbol>
    override val typeParameters get() = typeParams
    override val valueParameters get() = params
}

internal class KcConstructorSymbol(val info: KcInfo) : StubKaConstructorSymbol() {
    override val token get() = info.round.token
    override val psi get() = info.psi
    override val origin get() = info.origin
    override val annotations get() = info.annotations
    override val visibility get() = info.visibility
    override val compilerVisibility get() = info.compilerVisibility
    override val returnType: KaType
        get() = (info.parent as? KcClassSymbol)?.selfType ?: KcErrorType(info.round, "")
    override val containingClassId get() = (info.parent as? KcClassSymbol)?.classId
    override val isPrimary get() = info.decl.has(KcDecl.PRIMARY)
    override val isExpect get() = (info.parent as? KcClassSymbol)?.isExpect == true
    override val isActual get() = info.decl.has(KcDecl.ACTUAL)
    override val isExternal get() = info.decl.has(KcDecl.EXTERNAL)
    override val hasStableParameterNames get() = !info.isJava
    override fun createPointer(): KaSymbolPointer<KaConstructorSymbol> = info.pointer()
    override val typeParameters: List<KaTypeParameterSymbol> get() = (info.parent as? KcClassSymbol)?.typeParameters.orEmpty()

    lateinit var params: List<KaValueParameterSymbol>
    override val valueParameters get() = params
}

/**
 * A value parameter; [default] is an annotation parameter's default value,
 * which KSP asks a library parameter for (`KcExtras.javaAnnotationDefault`).
 */
internal class KcValueParameterSymbol(val info: KcInfo, val default: KaAnnotationValue? = null) : StubKaValueParameterSymbol() {
    override val token get() = info.round.token
    override val psi get() = info.psi
    override val origin get() = info.origin
    override val annotations get() = info.annotations
    override val name get() = info.name
    override val visibility get() = KaSymbolVisibility.LOCAL
    override val compilerVisibility get() = Visibilities.Local
    override val returnType: KaType by lazy { info.type(info.decl.type) }
    override val isVararg get() = info.decl.has(KcDecl.VARARG)
    override val isCrossinline get() = info.decl.has(KcDecl.CROSSINLINE)
    override val isNoinline get() = info.decl.has(KcDecl.NOINLINE)
    override val hasDeclaredDefaultValue get() = info.decl.has(KcDecl.HAS_DEFAULT)
    override val hasDefaultValue get() = hasDeclaredDefaultValue
    override val isImplicitLambdaParameter get() = false
    override fun createPointer(): KaSymbolPointer<KaValueParameterSymbol> = info.pointer()
}

internal class KcTypeParameterSymbol(val info: KcInfo) : StubKaTypeParameterSymbol() {
    override val token get() = info.round.token
    override val psi get() = info.psi
    override val origin get() = info.origin
    override val annotations get() = info.annotations
    override val name get() = info.name
    override val location get() = KaSymbolLocation.LOCAL
    override val upperBounds: List<KaType> by lazy {
        info.decl.bounds.map { info.type(it) }.ifEmpty { listOf(KcClassType(info.round, ClassId.fromString("kotlin/Any"), marked = true)) }
    }
    override val isReified get() = info.decl.has(KcDecl.REIFIED)
    override val variance: Variance
        get() = when (info.decl.variance) {
            KcDecl.IN -> Variance.IN_VARIANCE
            KcDecl.OUT -> Variance.OUT_VARIANCE
            else -> Variance.INVARIANT
        }
    override fun createPointer(): KaSymbolPointer<KaTypeParameterSymbol> = info.pointer()
}

internal class KcPropertySymbol(val info: KcInfo) : StubKaKotlinPropertySymbol() {
    override val token get() = info.round.token
    override val psi get() = info.psi
    override val origin get() = info.origin
    override val annotations get() = info.annotations
    override val name get() = info.name
    override val callableId get() = info.callableId(name)
    override val visibility get() = info.visibility
    override val compilerVisibility get() = info.compilerVisibility
    override val location get() = info.location
    override val modality get() = info.modality
    override val isVal get() = !info.decl.has(KcDecl.MUTABLE)
    override val isFromPrimaryConstructor get() = info.psi is KtParameter
    override val isConst get() = info.decl.has(KcDecl.CONST)
    override val isLateInit get() = info.decl.has(KcDecl.LATEINIT)
    override val isOverride get() = info.decl.has(KcDecl.OVERRIDE)
    override val isExternal get() = info.decl.has(KcDecl.EXTERNAL)
    override val isExpect get() = info.decl.has(KcDecl.EXPECT)
    override val isActual get() = info.decl.has(KcDecl.ACTUAL)
    override val isDelegated get() = (info.psi as? KtProperty)?.hasDelegate() == true
    override val isExtension get() = info.decl.receiver != null
    override val isStatic get() = info.decl.has(KcDecl.STATIC)
    override val receiverParameter: KaReceiverParameterSymbol? by lazy { info.receiver(this) }
    override val returnType: KaType by lazy { info.type(info.decl.type) }
    override val contextReceivers: List<KaContextReceiver> get() = emptyList()
    override val contextParameters: List<KaContextParameterSymbol> get() = emptyList()
    override val typeParameters: List<KaTypeParameterSymbol> get() = typeParams
    override val hasGetter get() = true
    override val hasSetter get() = !isVal
    override val hasBackingField get() = info.decl.has(KcDecl.BACKING_FIELD)
    override val backingFieldSymbol: KaBackingFieldSymbol? get() = backingField
    override fun createPointer(): KaSymbolPointer<KaKotlinPropertySymbol> = info.pointer()

    lateinit var typeParams: List<KaTypeParameterSymbol>
    lateinit var getterSymbol: KcGetterSymbol
    var setterSymbol: KcSetterSymbol? = null
    var backingField: KcBackingFieldSymbol? = null
    override val getter get() = getterSymbol
    override val setter get() = setterSymbol
}

internal class KcGetterSymbol(val info: KcInfo, val property: KcPropertySymbol) : StubKaPropertyGetterSymbol() {
    override val token get() = info.round.token
    override val psi get() = info.psi
    override val origin get() = info.origin
    override val annotations get() = info.annotations
    override val callableId: CallableId? get() = null
    override val visibility get() = property.visibility
    override val compilerVisibility get() = property.compilerVisibility
    override val modality get() = property.modality
    override val returnType get() = property.returnType
    override val isDefault get() = info.psi == null
    override val isNotDefault get() = !isDefault
    override val hasBody get() = (info.psi as? KtPropertyAccessor)?.hasBody() == true
    override val isInline get() = info.has(KtTokens.INLINE_KEYWORD)
    override val isExternal get() = info.has(KtTokens.EXTERNAL_KEYWORD)
    override val isOverride get() = property.isOverride
    override val receiverParameter get() = property.receiverParameter
    override fun createPointer(): KaSymbolPointer<KaPropertyGetterSymbol> = info.pointer()
}

internal class KcSetterSymbol(val info: KcInfo, val property: KcPropertySymbol) : StubKaPropertySetterSymbol() {
    override val token get() = info.round.token
    override val psi get() = info.psi
    override val origin get() = info.origin
    override val annotations get() = info.annotations
    override val callableId: CallableId? get() = null
    override val visibility get() = property.visibility
    override val compilerVisibility get() = property.compilerVisibility
    override val modality get() = property.modality
    override val returnType get() = info.unit
    override val isDefault get() = info.psi == null
    override val isNotDefault get() = !isDefault
    override val hasBody get() = (info.psi as? KtPropertyAccessor)?.hasBody() == true
    override val hasStableParameterNames get() = true
    override val isInline get() = info.has(KtTokens.INLINE_KEYWORD)
    override val isExternal get() = info.has(KtTokens.EXTERNAL_KEYWORD)
    override val isOverride get() = property.isOverride
    override val receiverParameter get() = property.receiverParameter
    override fun createPointer(): KaSymbolPointer<KaPropertySetterSymbol> = info.pointer()

    lateinit var parameterSymbol: KaValueParameterSymbol
    override val parameter get() = parameterSymbol
}

internal class KcEnumEntrySymbol(val info: KcInfo) : StubKaEnumEntrySymbol() {
    override val token get() = info.round.token
    override val psi get() = info.psi
    override val origin get() = info.origin
    override val annotations get() = info.annotations
    override val name get() = info.name
    override val callableId get() = info.callableId(name)
    override val returnType: KaType by lazy { info.type(info.decl.type) }
    override val isExpect get() = (info.parent as? KcClassSymbol)?.isExpect == true
    override val isExternal get() = false
    override fun createPointer(): KaSymbolPointer<KaEnumEntrySymbol> = info.pointer()
}

internal class KcTypeAliasSymbol(val info: KcInfo) : StubKaTypeAliasSymbol() {
    override val token get() = info.round.token
    override val psi get() = info.psi
    override val origin get() = info.origin
    override val annotations get() = info.annotations
    override val name get() = info.name
    override val classId: ClassId? = ClassId.fromString(info.decl.classId)
    override val visibility get() = info.visibility
    override val compilerVisibility get() = info.compilerVisibility
    override val location get() = info.location
    override val expandedType: KaType by lazy { info.type(info.decl.type) }
    override val isExpect get() = info.decl.has(KcDecl.EXPECT)
    override val isActual get() = info.decl.has(KcDecl.ACTUAL)
    override fun createPointer(): KaSymbolPointer<KaTypeAliasSymbol> = info.pointer()

    lateinit var typeParams: List<KaTypeParameterSymbol>
    override val typeParameters get() = typeParams
}

/** A field of a Java library class. */
internal class KcJavaFieldSymbol(val info: KcInfo) : StubKaJavaFieldSymbol() {
    override val token get() = info.round.token
    override val psi: PsiElement? get() = null
    override val origin get() = info.origin
    override val annotations get() = info.annotations
    override val name get() = info.name
    override val callableId get() = info.callableId(name)
    override val visibility get() = info.visibility
    override val compilerVisibility get() = info.compilerVisibility
    override val returnType: KaType by lazy { info.type(info.decl.type) }
    override val isVal get() = !info.decl.has(KcDecl.MUTABLE)
    override val isStatic get() = info.decl.has(KcDecl.STATIC)
    override fun createPointer(): KaSymbolPointer<KaJavaFieldSymbol> = info.pointer()
}

/** The receiver of an extension callable, [owner]; [returnType] is the receiver's type. */
/**
 * The backing field of a property: the property's annotations with use-site
 * target field or delegate, written or implied.
 */
internal class KcBackingFieldSymbol(val info: KcInfo, override val owningProperty: KcPropertySymbol) : StubKaBackingFieldSymbol() {
    override val token get() = info.round.token
    override val psi: PsiElement? get() = null
    override val annotations get() = info.annotations
    override val returnType: KaType get() = owningProperty.returnType
    override val isVal get() = owningProperty.isVal
    override val isNotDefault get() = false
    override fun createPointer(): KaSymbolPointer<KaBackingFieldSymbol> = info.pointer()
    override fun equals(other: Any?) = other is KcBackingFieldSymbol && other.owningProperty == owningProperty
    override fun hashCode() = owningProperty.hashCode() + 11
}

internal class KcReceiverParameterSymbol(
    val info: KcInfo,
    override val owningCallableSymbol: KaCallableSymbol,
    override val returnType: KaType,
) : StubKaReceiverParameterSymbol() {
    override val token get() = info.round.token
    override val psi: PsiElement? get() = (info.psi as? KtCallableDeclaration)?.receiverTypeReference
    override val origin get() = info.origin
    override val annotations: KaAnnotationList get() = KcAnnotationList(emptyList(), info.round.token)
    override fun createPointer(): KaSymbolPointer<KaReceiverParameterSymbol> = info.pointer()
    override fun equals(other: Any?) = other is KcReceiverParameterSymbol && other.owningCallableSymbol == owningCallableSymbol
    override fun hashCode() = owningCallableSymbol.hashCode() + 7
}

/** A class kc doesn't have, known only by its ID. */
internal class KcLibraryClassSymbol(val round: KcRound, override val classId: ClassId) : StubKaNamedClassSymbol() {
    override val token get() = round.token
    override val psi: PsiElement? get() = null
    override val origin get() = KaSymbolOrigin.LIBRARY
    override val name get() = classId.shortClassName
    override val location get() = KaSymbolLocation.TOP_LEVEL
    override fun createPointer(): KaSymbolPointer<KaNamedClassSymbol> = KcPointer("class:" + classId.asString())
    override fun toString() = "KcLibraryClassSymbol($classId)"
}

internal class KcAnnotationApplication(
    private val round: KcRound,
    override val classId: ClassId?,
    override val psi: KtCallElement?,
    override val useSiteTarget: AnnotationUseSiteTarget?,
    override val arguments: List<KaNamedAnnotationValue>,
) : StubKaAnnotation() {
    override val token get() = round.token
    override fun toString() = "@$classId" + arguments.joinToString(", ", "(", ")")
}

internal class KcAnnotationList(
    private val list: List<KaAnnotation>,
    override val token: KaLifetimeToken,
) : KaAnnotationList, List<KaAnnotation> by list {
    override val classIds: Collection<ClassId> get() = list.mapNotNull { it.classId }.distinct()
    override fun contains(classId: ClassId) = list.any { it.classId == classId }
    override fun get(classId: ClassId) = list.filter { it.classId == classId }
}

/** A scope over a fixed list of declarations. */
internal class KcScope(
    private val decls: List<KaDeclarationSymbol>,
    override val token: KaLifetimeToken,
) : KaScope {
    override val declarations get() = decls.asSequence()
    override fun declarations(nameFilter: (Name) -> Boolean) = decls.asSequence().filter { nameOf(it)?.let(nameFilter) == true }
    override fun declarations(names: Collection<Name>) = declarations { it in names }
    override fun declarations(vararg names: Name) = declarations(names.toList())

    override val callables get() = decls.asSequence().filterIsInstance<KaCallableSymbol>().filter { it !is KaConstructorSymbol }
    override fun callables(nameFilter: (Name) -> Boolean) = callables.filter { nameOf(it)?.let(nameFilter) == true }
    override fun callables(names: Collection<Name>) = callables { it in names }
    override fun callables(vararg names: Name) = callables(names.toList())

    override val classifiers get() = decls.asSequence().filterIsInstance<KaClassifierSymbol>()
    override fun classifiers(nameFilter: (Name) -> Boolean) = classifiers.filter { nameOf(it)?.let(nameFilter) == true }
    override fun classifiers(names: Collection<Name>) = classifiers { it in names }
    override fun classifiers(vararg names: Name) = classifiers(names.toList())

    override val constructors get() = decls.asSequence().filterIsInstance<KaConstructorSymbol>()
    override fun getPackageSymbols(nameFilter: (Name) -> Boolean) = emptySequence<KaPackageSymbol>()
    override fun getPossibleCallableNames() = callables.mapNotNull { nameOf(it) }.toSet()
    override fun getPossibleClassifierNames() = classifiers.mapNotNull { nameOf(it) }.toSet()

    private fun nameOf(s: KaSymbol): Name? = when (s) {
        is KaConstructorSymbol -> SpecialNames.INIT
        is KaNamedSymbol -> s.name
        else -> null
    }
}

/** The use-site target an annotation's Go model names, e.g. "get". */
internal fun useSiteTarget(name: String): AnnotationUseSiteTarget? =
    if (name.isEmpty()) null else AnnotationUseSiteTarget.entries.firstOrNull { it.renderName == name }

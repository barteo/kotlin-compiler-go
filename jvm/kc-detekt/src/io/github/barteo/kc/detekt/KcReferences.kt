package io.github.barteo.kc.detekt

import com.intellij.psi.PsiElement
import com.intellij.psi.PsiReference
import com.intellij.psi.ResolveResult
import com.intellij.psi.impl.source.resolve.ResolveCache
import org.jetbrains.kotlin.idea.references.KDocReference
import org.jetbrains.kotlin.idea.references.KtArrayAccessReference
import org.jetbrains.kotlin.idea.references.KtCollectionLiteralReference
import org.jetbrains.kotlin.idea.references.KtConstructorDelegationReference
import org.jetbrains.kotlin.idea.references.KtInvokeFunctionReference
import org.jetbrains.kotlin.idea.references.KtReference
import org.jetbrains.kotlin.idea.references.KtSimpleNameReference
import org.jetbrains.kotlin.kdoc.psi.impl.KDocName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.psi.KtImplementationDetail
import org.jetbrains.kotlin.psi.KotlinReferenceProvidersService
import org.jetbrains.kotlin.psi.KtArrayAccessExpression
import org.jetbrains.kotlin.psi.KtCallExpression
import org.jetbrains.kotlin.psi.KtCollectionLiteralExpression
import org.jetbrains.kotlin.psi.KtConstructorDelegationReferenceExpression
import org.jetbrains.kotlin.psi.KtImportAlias
import org.jetbrains.kotlin.psi.KtSimpleNameExpression

/**
 * The PSI references of the session: a name's `mainReference`, which rules
 * such as UnusedImport take before they call `KaSession.resolveToSymbol`.
 * kc answers that from the reference's element, so the references only
 * carry their element; resolving one through PSI finds nothing.
 */
internal class KcReferenceProviders : KotlinReferenceProvidersService() {
    override fun getReferences(psi: PsiElement): Array<PsiReference> = when (psi) {
        is KtSimpleNameExpression -> arrayOf(KcNameReference(psi))
        is KDocName -> arrayOf(KcKDocReference(psi))
        // Every call has an invoke reference, as in the Analysis API.
        is KtCallExpression -> arrayOf(KcInvokeReference(psi))
        is KtArrayAccessExpression -> arrayOf(KcArrayAccessReference(psi))
        is KtCollectionLiteralExpression -> arrayOf(KcCollectionLiteralReference(psi))
        is KtConstructorDelegationReferenceExpression -> arrayOf(KcConstructorDelegationReference(psi))
        else -> PsiReference.EMPTY_ARRAY
    }
}

/** Resolves to nothing: kc answers resolveToSymbol from the element. */
private object Nothing : ResolveCache.PolyVariantResolver<KtReference> {
    override fun resolve(ref: KtReference, incompleteCode: Boolean): Array<ResolveResult> = ResolveResult.EMPTY_ARRAY
}

@OptIn(KtImplementationDetail::class)
internal class KcNameReference(e: KtSimpleNameExpression) : KtSimpleNameReference(e) {
    override fun isReferenceToViaExtension(element: PsiElement) = false
    override fun getImportAlias(): KtImportAlias? = null
    override fun handleElementRename(newElementName: String): PsiElement = expression
    override fun bindToElement(element: PsiElement): PsiElement = expression
    override val resolvesByNames: Collection<Name> get() = listOf(expression.getReferencedNameAsName())
    override val resolver: ResolveCache.PolyVariantResolver<KtReference> get() = Nothing
}

@OptIn(KtImplementationDetail::class)
internal class KcKDocReference(e: KDocName) : KDocReference(e) {
    override fun handleElementRename(newElementName: String): PsiElement = element
    override fun bindToElement(element: PsiElement): PsiElement = this.element
    override val resolver: ResolveCache.PolyVariantResolver<KtReference> get() = Nothing
}

@OptIn(KtImplementationDetail::class)
internal class KcInvokeReference(e: KtCallExpression) : KtInvokeFunctionReference(e) {
    override fun handleElementRename(newElementName: String): PsiElement = element
    override fun bindToElement(element: PsiElement): PsiElement = this.element
    override val resolver: ResolveCache.PolyVariantResolver<KtReference> get() = Nothing
}

@OptIn(KtImplementationDetail::class)
internal class KcArrayAccessReference(e: KtArrayAccessExpression) : KtArrayAccessReference(e) {
    override fun handleElementRename(newElementName: String): PsiElement = element
    override fun bindToElement(element: PsiElement): PsiElement = this.element
    override val resolver: ResolveCache.PolyVariantResolver<KtReference> get() = Nothing
}

@OptIn(KtImplementationDetail::class)
internal class KcCollectionLiteralReference(e: KtCollectionLiteralExpression) : KtCollectionLiteralReference(e) {
    override fun handleElementRename(newElementName: String): PsiElement = element
    override fun bindToElement(element: PsiElement): PsiElement = this.element
    override val resolver: ResolveCache.PolyVariantResolver<KtReference> get() = Nothing
}

@OptIn(KtImplementationDetail::class)
internal class KcConstructorDelegationReference(e: KtConstructorDelegationReferenceExpression) : KtConstructorDelegationReference(e) {
    override fun handleElementRename(newElementName: String): PsiElement = element
    override fun bindToElement(element: PsiElement): PsiElement = this.element
    override val resolver: ResolveCache.PolyVariantResolver<KtReference> get() = Nothing
}

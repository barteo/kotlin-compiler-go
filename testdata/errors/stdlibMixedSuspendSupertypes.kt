// A class that mixes suspend and non-suspend function types is reported
// once, at its supertypes, not again for its invoke.

interface ISuper : () -> Unit
interface SISuper : suspend () -> Unit

class C : suspend () -> Unit, ISuper { // ERROR mixing suspend and non-suspend supertypes
    override suspend fun invoke() {}
}

object O : SISuper, ISuper { // ERROR mixing suspend and non-suspend supertypes
    override suspend fun invoke() {}
}

class Plain : ISuper {
    override suspend fun invoke() {} // ERROR suspend function 'invoke' cannot override non-suspend function
}

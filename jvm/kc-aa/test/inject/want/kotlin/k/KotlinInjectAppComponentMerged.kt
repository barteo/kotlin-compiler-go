package k

import amazon.lastmile.inject.KAppIdComponent
import amazon.lastmile.inject.KFakeAuthenticator
import amazon.lastmile.inject.KFakeObject
import amazon.lastmile.inject.KImpl
import amazon.lastmile.inject.KLoggingInterceptor
import amazon.lastmile.inject.KRendererComponentFactory
import amazon.lastmile.inject.KRetryInterceptor
import amazon.lastmile.inject.KStoreComponent
import amazon.lastmile.inject.KStringLoader

public interface KotlinInjectAppComponentMerged : RendererComponentFinalKotlinInjectAppComponent.Factory,
    KLoggingInterceptor, KRetryInterceptor, KImpl, KStringLoader, KFakeObject, KFakeAuthenticator,
    KRendererComponentFactory, KAppIdComponent, KStoreComponent

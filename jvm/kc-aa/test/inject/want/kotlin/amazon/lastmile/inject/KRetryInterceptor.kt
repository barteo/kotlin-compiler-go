package amazon.lastmile.inject

import k.Interceptor
import k.RetryInterceptor
import me.tatarka.inject.annotations.IntoSet
import me.tatarka.inject.annotations.Provides
import software.amazon.lastmile.kotlin.inject.anvil.`internal`.Origin

@Origin(value = RetryInterceptor::class)
public interface KRetryInterceptor {
  @Provides
  @IntoSet
  public fun provideRetryInterceptorInterceptorMultibinding(retryInterceptor: RetryInterceptor): Interceptor = retryInterceptor
}

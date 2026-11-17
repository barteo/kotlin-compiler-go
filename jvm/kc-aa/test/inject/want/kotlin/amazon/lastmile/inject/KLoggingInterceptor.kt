package amazon.lastmile.inject

import k.Interceptor
import k.LoggingInterceptor
import me.tatarka.inject.annotations.IntoSet
import me.tatarka.inject.annotations.Provides
import software.amazon.lastmile.kotlin.inject.anvil.`internal`.Origin

@Origin(value = LoggingInterceptor::class)
public interface KLoggingInterceptor {
  @Provides
  @IntoSet
  public fun provideLoggingInterceptorInterceptorMultibinding(loggingInterceptor: LoggingInterceptor): Interceptor = loggingInterceptor
}

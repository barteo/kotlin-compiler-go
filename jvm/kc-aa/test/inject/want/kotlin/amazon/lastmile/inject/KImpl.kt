package amazon.lastmile.inject

import k.Base
import k.Impl
import k.Other
import me.tatarka.inject.annotations.IntoSet
import me.tatarka.inject.annotations.Provides
import software.amazon.lastmile.kotlin.inject.anvil.`internal`.Origin

@Origin(value = Impl::class)
public interface KImpl {
  @Provides
  public fun provideImplBase(`impl`: Impl): Base = impl

  @Provides
  @IntoSet
  public fun provideImplOtherMultibinding(`impl`: Impl): Other = impl
}

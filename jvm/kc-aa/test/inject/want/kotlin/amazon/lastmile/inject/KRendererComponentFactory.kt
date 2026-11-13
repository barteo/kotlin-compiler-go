package amazon.lastmile.inject

import k.RendererComponent
import me.tatarka.inject.annotations.Provides
import software.amazon.lastmile.kotlin.inject.anvil.`internal`.Origin
import software.amazon.lastmile.kotlin.inject.anvil.`internal`.Subcomponent

@Origin(value = RendererComponent.Factory::class)
@Subcomponent
public interface KRendererComponentFactory : RendererComponent.Factory {
  @Provides
  public fun provideRendererComponentFactory(): RendererComponent.Factory = this
}

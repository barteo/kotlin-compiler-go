package k

import me.tatarka.inject.annotations.Component
import me.tatarka.inject.annotations.Provides
import software.amazon.lastmile.kotlin.inject.anvil.MergeComponent
import software.amazon.lastmile.kotlin.inject.anvil.SingleIn
import software.amazon.lastmile.kotlin.inject.anvil.`internal`.Origin

@Component
@MergeComponent(scope = LoggedInScope::class)
@SingleIn(scope = LoggedInScope::class)
@Origin(value = RendererComponent::class)
public abstract class RendererComponentFinalKotlinInjectAppComponent(
  @Component
  public val parentComponent: KotlinInjectAppComponent,
  @get:Provides
  public val user: UserId,
) : RendererComponent,
    RendererComponentFinalKotlinInjectAppComponentMerged {
  public interface Factory : RendererComponent.Factory {
    override fun createRendererComponent(user: UserId): RendererComponent = RendererComponentFinalKotlinInjectAppComponent::class.create(this as KotlinInjectAppComponent, user)
  }
}

package k

import kotlin.reflect.KClass
import me.tatarka.inject.annotations.Component
import software.amazon.lastmile.kotlin.inject.anvil.AppScope
import software.amazon.lastmile.kotlin.inject.anvil.MergeComponent
import software.amazon.lastmile.kotlin.inject.anvil.SingleIn
import software.amazon.lastmile.kotlin.inject.anvil.`internal`.Origin

@Origin(value = AppComponent::class)
@Component
@MergeComponent(
  scope = AppScope::class,
  exclude = arrayOf(ExcludedComponent::class),
)
@SingleIn(scope = AppScope::class)
public abstract class KotlinInjectAppComponent(
  configDelegate: Config,
) : AppComponent(configDelegate),
    KotlinInjectAppComponentMerged {
  public companion object
}

public fun KClass<AppComponent>.create(configDelegate: Config): AppComponent = KotlinInjectAppComponent::class.create(configDelegate)

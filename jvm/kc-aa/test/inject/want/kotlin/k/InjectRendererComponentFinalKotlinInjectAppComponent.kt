package k

import kotlin.reflect.KClass
import me.tatarka.inject.`internal`.LazyMap
import me.tatarka.inject.`internal`.ScopedComponent

public fun KClass<RendererComponentFinalKotlinInjectAppComponent>.create(parentComponent: KotlinInjectAppComponent, user: UserId): RendererComponentFinalKotlinInjectAppComponent = InjectRendererComponentFinalKotlinInjectAppComponent(parentComponent, user)

public class InjectRendererComponentFinalKotlinInjectAppComponent(
  parentComponent: KotlinInjectAppComponent,
  user: UserId,
) : RendererComponentFinalKotlinInjectAppComponent(parentComponent, user),
    ScopedComponent {
  override val _scoped: LazyMap = LazyMap()

  override val renderer: Renderer
    get() = _scoped.get("k.Renderer") {
      Renderer(
        user = user,
        auth = parentComponent.auth
      )
    }
}

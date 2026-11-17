package k

import kotlin.reflect.KClass
import me.tatarka.inject.`internal`.LazyMap
import me.tatarka.inject.`internal`.ScopedComponent

public fun KClass<RendererComponentFinalPlainComponent>.create(parentComponent: PlainComponent, user: UserId): RendererComponentFinalPlainComponent = InjectRendererComponentFinalPlainComponent(parentComponent, user)

public class InjectRendererComponentFinalPlainComponent(
  parentComponent: PlainComponent,
  user: UserId,
) : RendererComponentFinalPlainComponent(parentComponent, user),
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

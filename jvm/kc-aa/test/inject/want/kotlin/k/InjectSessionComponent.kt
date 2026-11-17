package k

import kotlin.reflect.KClass
import me.tatarka.inject.`internal`.LazyMap
import me.tatarka.inject.`internal`.ScopedComponent

public fun KClass<SessionComponent>.create(parent: GraphComponent): SessionComponent = InjectSessionComponent(parent)

public class InjectSessionComponent(
  parent: GraphComponent,
) : SessionComponent(parent),
    ScopedComponent {
  override val _scoped: LazyMap = LazyMap()

  override val cache: Cache
    get() = _scoped.get("k.Cache") {
      Cache(
        token = parent.token,
        service = parent.service(
          clock = Clock()
        )
      )
    }
}

package k

import kotlin.reflect.KClass

public fun KClass<SelfComponent>.create(): SelfComponent = InjectSelfComponent()

public class InjectSelfComponent : SelfComponent {
  override val counter: Counter
    get() = Counter(
      clock = Clock()
    )
}

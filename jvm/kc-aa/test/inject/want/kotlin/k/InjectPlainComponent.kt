package k

import kotlin.reflect.KClass
import me.tatarka.inject.`internal`.LazyMap
import me.tatarka.inject.`internal`.ScopedComponent

public fun KClass<PlainComponent>.create(config: Config): PlainComponent = InjectPlainComponent(config)

public fun KClass<PlainComponent>.create(): PlainComponent = InjectPlainComponent()

public class InjectPlainComponent : PlainComponent, ScopedComponent {
  override val _scoped: LazyMap = LazyMap()

  override val auth: Authenticator
    get() = provideFakeAuthenticatorAuthenticator(
      fakeAuthenticator = FakeAuthenticator()
    )

  override val renderers: RendererComponent.Factory
    get() = provideRendererComponentFactory()

  public constructor(config: Config) : super(config)

  public constructor() : super()
}

package k

import kotlin.collections.Set
import kotlin.reflect.KClass
import me.tatarka.inject.`internal`.LazyMap
import me.tatarka.inject.`internal`.ScopedComponent

public fun KClass<KotlinInjectAppComponent>.create(configDelegate: Config): KotlinInjectAppComponent = InjectKotlinInjectAppComponent(configDelegate)

public class InjectKotlinInjectAppComponent(
  configDelegate: Config,
) : KotlinInjectAppComponent(configDelegate),
    ScopedComponent {
  override val _scoped: LazyMap = LazyMap()

  override val auth: Authenticator
    get() = provideFakeAuthenticatorAuthenticator(
      fakeAuthenticator = FakeAuthenticator()
    )

  override val interceptors: Set<Interceptor>
    get() = setOf(
      provideLoggingInterceptorInterceptorMultibinding(
        loggingInterceptor = LoggingInterceptor()
      ),
      provideRetryInterceptorInterceptorMultibinding(
        retryInterceptor = RetryInterceptor(
          retries = retries()
        )
      )
    )

  override val screen: Screen
    get() = Screen(
      loaderFactory = { arg0 ->
        provideStringLoaderLoader(
          path = arg0,
          realFactory = { arg0_ ->
            StringLoader(
              path = arg0_
            )
          }
        )
      },
      store = store(
        impl = NumberStore()
      ),
      lazyAuth = lazy {
        auth
      },
      interceptors = interceptors
    )

  override val base: Base
    get() = provideImplBase(
      impl = Impl()
    )

  override val others: Set<Other>
    get() = setOf(
      provideImplOtherMultibinding(
        impl = Impl()
      )
    )

  override val fake: Fake
    get() = provideFakeObjectFake(
      fakeObject = FakeObject
    )
}

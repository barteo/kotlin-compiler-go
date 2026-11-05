package d;

import dagger.internal.DaggerGenerated;
import dagger.internal.DoubleCheck;
import dagger.internal.InstanceFactory;
import dagger.internal.Preconditions;
import dagger.internal.Provider;
import java.util.Collections;
import java.util.Set;
import javax.annotation.processing.Generated;

@DaggerGenerated
@Generated(
    value = "dagger.internal.codegen.ComponentProcessor",
    comments = "https://dagger.dev"
)
@SuppressWarnings({
    "unchecked",
    "rawtypes",
    "KotlinInternal",
    "KotlinInternalInJava",
    "cast",
    "deprecation",
    "nullness:initialization.field.uninitialized"
})
public final class DaggerAppComponent {
  private DaggerAppComponent() {
  }

  public static AppComponent.Factory factory() {
    return new Factory();
  }

  private static final class Factory implements AppComponent.Factory {
    @Override
    public AppComponent create(int port) {
      Preconditions.checkNotNull(port);
      return new AppComponentImpl(port);
    }
  }

  private static final class AppComponentImpl implements AppComponent {
    private final AppComponentImpl appComponentImpl = this;

    Provider<Integer> portProvider;

    Provider<Repo> repoProvider;

    AppComponentImpl(Integer portParam) {

      initialize(portParam);

    }

    MemStore memStore() {
      return new MemStore(DoubleCheck.lazy(repoProvider), Clock_Factory.create());
    }

    String plugin() {
      return NetModule_PluginFactory.plugin(new Clock());
    }

    @SuppressWarnings("unchecked")
    private void initialize(final Integer portParam) {
      this.portProvider = InstanceFactory.create(portParam);
      this.repoProvider = DoubleCheck.provider(Repo_Factory.create(Clock_Factory.create(), NetModule_UrlFactory.create(), portProvider));
    }

    @Override
    public Store store() {
      return memStore();
    }

    @Override
    public Repo getRepo() {
      return repoProvider.get();
    }

    @Override
    public Set<String> plugins() {
      return Collections.<String>singleton(plugin());
    }

    @Override
    public void inject(Target t) {
      injectTarget(t);
    }

    private Target injectTarget(Target instance) {
      Target_MembersInjector.injectClock(instance, new Clock());
      Target_MembersInjector.injectUrl(instance, NetModule_UrlFactory.url());
      return instance;
    }
  }
}

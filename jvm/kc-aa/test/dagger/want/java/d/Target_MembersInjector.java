package d;

import dagger.MembersInjector;
import dagger.internal.DaggerGenerated;
import dagger.internal.InjectedFieldSignature;
import dagger.internal.Provider;
import dagger.internal.QualifierMetadata;
import javax.annotation.processing.Generated;
import javax.inject.Named;

@QualifierMetadata("javax.inject.Named")
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
public final class Target_MembersInjector implements MembersInjector<Target> {
  private final Provider<Clock> clockProvider;

  private final Provider<String> urlProvider;

  private Target_MembersInjector(Provider<Clock> clockProvider, Provider<String> urlProvider) {
    this.clockProvider = clockProvider;
    this.urlProvider = urlProvider;
  }

  @Override
  public void injectMembers(Target instance) {
    injectClock(instance, clockProvider.get());
    injectUrl(instance, urlProvider.get());
  }

  public static MembersInjector<Target> create(Provider<Clock> clockProvider,
      Provider<String> urlProvider) {
    return new Target_MembersInjector(clockProvider, urlProvider);
  }

  @InjectedFieldSignature("d.Target.clock")
  public static void injectClock(Target instance, Clock clock) {
    instance.clock = clock;
  }

  @InjectedFieldSignature("d.Target.url")
  @Named("url")
  public static void injectUrl(Target instance, String url) {
    instance.url = url;
  }
}

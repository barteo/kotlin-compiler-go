package a;

import dagger.MembersInjector;
import dagger.internal.DaggerGenerated;
import dagger.internal.InjectedFieldSignature;
import dagger.internal.Provider;
import dagger.internal.QualifierMetadata;
import javax.annotation.processing.Generated;

@QualifierMetadata
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
public final class Panel_MembersInjector implements MembersInjector<Panel> {
  private final Provider<Bar> barProvider;

  private Panel_MembersInjector(Provider<Bar> barProvider) {
    this.barProvider = barProvider;
  }

  @Override
  public void injectMembers(Panel instance) {
    injectBar(instance, barProvider.get());
  }

  public static MembersInjector<Panel> create(Provider<Bar> barProvider) {
    return new Panel_MembersInjector(barProvider);
  }

  @InjectedFieldSignature("a.Panel.bar")
  public static void injectBar(Panel instance, Bar bar) {
    instance.bar = bar;
  }
}

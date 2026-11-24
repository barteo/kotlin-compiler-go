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
public final class Screen_MembersInjector implements MembersInjector<Screen> {
  private final Provider<Foo> fooProvider;

  private Screen_MembersInjector(Provider<Foo> fooProvider) {
    this.fooProvider = fooProvider;
  }

  @Override
  public void injectMembers(Screen instance) {
    injectFoo(instance, fooProvider.get());
  }

  public static MembersInjector<Screen> create(Provider<Foo> fooProvider) {
    return new Screen_MembersInjector(fooProvider);
  }

  @InjectedFieldSignature("a.Screen.foo")
  public static void injectFoo(Screen instance, Foo foo) {
    instance.foo = foo;
  }
}

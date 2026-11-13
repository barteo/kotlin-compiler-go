package io.github.barteo.kc;

/**
 * The record of the JVM's libkc, registered by {@link KcBridge} in its MBeanServer. It is
 * loaded by the class loader that loaded libkc, so the MBeanServer finds that class loader
 * ({@link javax.management.MBeanServer#getClassLoaderFor}) and keeps it.
 */
public final class KcLibrary implements KcLibraryMBean {
  private final String library;
  private final String jar;

  public KcLibrary(String library, String jar) {
    this.library = library;
    this.jar = jar;
  }

  @Override
  public String getLibrary() {
    return library;
  }

  @Override
  public String getJar() {
    return jar;
  }

  @Override
  public String getAbiVersion() {
    // KcNative checked, when it loaded libkc, that libkc has this ABI.
    return KcNative.ABI_VERSION;
  }
}

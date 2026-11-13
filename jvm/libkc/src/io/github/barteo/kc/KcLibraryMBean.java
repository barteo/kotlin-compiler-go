package io.github.barteo.kc;

/** The management interface of {@link KcLibrary}. */
public interface KcLibraryMBean {
  /** The libkc loaded: the kc.lib property then, or "kc on java.library.path". */
  String getLibrary();

  /** The jar, or directory, whose KcNative loaded it. */
  String getJar();

  /** The ABI version of the loaded libkc. */
  String getAbiVersion();
}

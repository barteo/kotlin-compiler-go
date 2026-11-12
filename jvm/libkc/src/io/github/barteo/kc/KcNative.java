package io.github.barteo.kc;

/**
 * The JNI entry point of libkc (cmd/libkc in kotlin-compiler-go).
 *
 * <p>The library is loaded from the path in the system property {@code kc.lib}, or else
 * as {@code kc} from {@code java.library.path}. Loading it checks that its ABI version is
 * the one this class was written for.
 */
public final class KcNative {
  /** The ABI version this class expects; internal/kcabi.Version on the Go side. */
  public static final String ABI_VERSION = "kc-aa/0";

  public static final int OP_VERSION = 1;
  public static final int OP_ECHO = 2;
  public static final int OP_PANIC = 3;

  static {
    String path = System.getProperty("kc.lib");
    if (path != null) {
      System.load(new java.io.File(path).getAbsolutePath());
    } else {
      System.loadLibrary("kc");
    }
    String v = new String(call(OP_VERSION, new byte[0]), java.nio.charset.StandardCharsets.UTF_8);
    if (!v.equals(ABI_VERSION)) {
      throw new UnsatisfiedLinkError("libkc has ABI " + v + ", expected " + ABI_VERSION);
    }
  }

  private KcNative() {}

  /**
   * Runs operation {@code op} on {@code request} and returns its response.
   *
   * @throws KcException if the operation fails
   * @throws KcPanicException if kc panicked, which is a bug in kc
   */
  public static native byte[] call(int op, byte[] request);
}

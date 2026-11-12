package io.github.barteo.kc;

/** kc panicked in an operation of libkc: a bug in kc. The message holds the Go stack. */
public class KcPanicException extends KcException {
  public KcPanicException(String message) {
    super(message);
  }
}

package io.github.barteo.kc;

/** An operation of libkc failed. */
public class KcException extends RuntimeException {
  public KcException(String message) {
    super(message);
  }
}

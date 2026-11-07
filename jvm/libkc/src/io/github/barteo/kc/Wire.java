package io.github.barteo.kc;

import java.io.ByteArrayOutputStream;
import java.nio.charset.StandardCharsets;

/**
 * The byte format of libkc's requests and responses; the twin of Go's
 * internal/kcabi/wire. A message is a sequence of values with no framing or
 * tags: both sides know an operation's layout.
 *
 * <ul>
 *   <li>Uint is an unsigned LEB128 varint, Int a zigzag one;
 *   <li>Bool is a byte, 0 or 1;
 *   <li>String and Bytes are a Uint length and the bytes, UTF-8 for String.
 * </ul>
 */
public final class Wire {
  private Wire() {}

  /** Builds a message. */
  public static final class Writer {
    private final ByteArrayOutputStream out = new ByteArrayOutputStream();

    public byte[] message() {
      return out.toByteArray();
    }

    public Writer writeUint(long x) {
      while ((x & ~0x7FL) != 0) {
        out.write((int) ((x & 0x7F) | 0x80));
        x >>>= 7;
      }
      out.write((int) x);
      return this;
    }

    public Writer writeInt(long x) {
      return writeUint((x << 1) ^ (x >> 63));
    }

    public Writer writeBool(boolean b) {
      out.write(b ? 1 : 0);
      return this;
    }

    public Writer writeBytes(byte[] b) {
      writeUint(b.length);
      out.write(b, 0, b.length);
      return this;
    }

    public Writer writeString(String s) {
      return writeBytes(s.getBytes(StandardCharsets.UTF_8));
    }
  }

  /** Reads a message. A malformed message throws {@link IllegalArgumentException}. */
  public static final class Reader {
    private final byte[] buf;
    private int pos;

    public Reader(byte[] buf) {
      this.buf = buf;
    }

    public boolean atEnd() {
      return pos == buf.length;
    }

    public long readUint() {
      long x = 0;
      for (int shift = 0; shift < 64; shift += 7) {
        if (pos >= buf.length) throw malformed();
        int b = buf[pos++] & 0xFF;
        x |= (long) (b & 0x7F) << shift;
        if ((b & 0x80) == 0) return x;
      }
      throw malformed();
    }

    public long readInt() {
      long u = readUint();
      return (u >>> 1) ^ -(u & 1);
    }

    public boolean readBool() {
      if (pos >= buf.length || (buf[pos] & 0xFF) > 1) throw malformed();
      return buf[pos++] == 1;
    }

    public byte[] readBytes() {
      int n = length();
      byte[] b = java.util.Arrays.copyOfRange(buf, pos, pos + n);
      pos += n;
      return b;
    }

    public String readString() {
      int n = length();
      String s = new String(buf, pos, n, StandardCharsets.UTF_8);
      pos += n;
      return s;
    }

    private int length() {
      long n = readUint();
      if (n > buf.length - pos) throw malformed();
      return (int) n;
    }

    private static IllegalArgumentException malformed() {
      return new IllegalArgumentException("wire: malformed message");
    }
  }
}

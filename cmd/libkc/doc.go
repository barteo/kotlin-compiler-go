// Command libkc builds kc's analysis as a shared library that a JVM loads
// through JNI, so that the Kotlin Analysis API implementation in jvm/ can
// answer queries in process (docs/analysis-api.md).
//
// Build it with cgo, the libkc build tag and the JDK's JNI headers:
//
//	CGO_CFLAGS="-I$JAVA_HOME/include -I$JAVA_HOME/include/linux" \
//		go build -tags libkc -buildmode=c-shared -o libkc.so ./cmd/libkc
//
// The library exports one JNI method, KcBridge.call(int op, byte[] request,
// int[] status) in package io.github.barteo.kc, which runs
// internal/kcabi.Call and returns its response, or the message of its
// error with status[0] set. It names no class of the JVM's, so the class
// loader that loads the library, which a JVM lets one do, can serve the
// callers of every other (jvm/libkc's KcNative). Without the libkc tag the
// command builds as a stub, so that go build ./... and go vet ./... work on
// hosts without a JDK.
package main

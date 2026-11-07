// Command libkc builds kc's analysis as a shared library that a JVM loads
// through JNI, so that the Kotlin Analysis API implementation in jvm/ can
// answer queries in process (docs/analysis-api.md).
//
// Build it with cgo, the libkc build tag and the JDK's JNI headers:
//
//	CGO_CFLAGS="-I$JAVA_HOME/include -I$JAVA_HOME/include/linux" \
//		go build -tags libkc -buildmode=c-shared -o libkc.so ./cmd/libkc
//
// The library exports one JNI method, KcNative.call(int op, byte[] request)
// in package io.github.barteo.kc, which runs internal/kcabi.Call and throws
// io.github.barteo.kc.KcException when it fails. Without the libkc tag the
// command builds as a stub, so that go build ./... and go vet ./... work on
// hosts without a JDK.
package main

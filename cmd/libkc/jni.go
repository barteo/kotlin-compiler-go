//go:build libkc

package main

/*
#include <jni.h>
#include <stdlib.h>

static jsize kcLen(JNIEnv *env, jbyteArray a) {
	return a ? (*env)->GetArrayLength(env, a) : 0;
}

static void kcRead(JNIEnv *env, jbyteArray a, jsize n, void *dst) {
	(*env)->GetByteArrayRegion(env, a, 0, n, (jbyte *)dst);
}

static jbyteArray kcNew(JNIEnv *env, const void *src, jsize n) {
	jbyteArray a = (*env)->NewByteArray(env, n);
	if (a != NULL && n > 0) {
		(*env)->SetByteArrayRegion(env, a, 0, n, (const jbyte *)src);
	}
	return a;
}

// kcThrow throws a KcException; panic tells a bug in kc from a bad request.
static void kcThrow(JNIEnv *env, const char *msg, int panic) {
	jclass c = (*env)->FindClass(env, panic ? "io/github/barteo/kc/KcPanicException" : "io/github/barteo/kc/KcException");
	if (c != NULL) {
		(*env)->ThrowNew(env, c, msg);
	}
}
*/
import "C"

import (
	"errors"
	"unsafe"

	_ "github.com/barteo/kotlin-compiler-go/internal/kaops" // registers the Analysis API operations
	"github.com/barteo/kotlin-compiler-go/internal/kcabi"
)

func main() {}

//export Java_io_github_barteo_kc_KcNative_call
func Java_io_github_barteo_kc_KcNative_call(env *C.JNIEnv, _ C.jclass, op C.jint, req C.jbyteArray) C.jbyteArray {
	n := C.kcLen(env, req)
	in := make([]byte, int(n))
	if n > 0 {
		C.kcRead(env, req, n, unsafe.Pointer(&in[0]))
	}
	out, err := kcabi.Call(kcabi.Op(op), in)
	if err != nil {
		var e *kcabi.Error
		panicked := errors.As(err, &e) && e.Panic
		msg := C.CString(err.Error())
		defer C.free(unsafe.Pointer(msg))
		p := C.int(0)
		if panicked {
			p = 1
		}
		C.kcThrow(env, msg, p)
		return 0
	}
	var src unsafe.Pointer
	if len(out) > 0 {
		src = unsafe.Pointer(&out[0])
	}
	return C.kcNew(env, src, C.jsize(len(out)))
}

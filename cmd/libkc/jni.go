//go:build libkc

package main

/*
#include <jni.h>

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

static void kcStatus(JNIEnv *env, jintArray a, jint v) {
	(*env)->SetIntArrayRegion(env, a, 0, 1, &v);
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

// The values KcBridge.call stores in status[0].
const (
	statusOK    = 0
	statusError = 1 // the response is the message of an error the request caused
	statusPanic = 2 // the response is the message of a panic, which is a bug in kc
)

//export Java_io_github_barteo_kc_KcBridge_call
func Java_io_github_barteo_kc_KcBridge_call(env *C.JNIEnv, _ C.jclass, op C.jint, req C.jbyteArray, status C.jintArray) C.jbyteArray {
	n := C.kcLen(env, req)
	in := make([]byte, int(n))
	if n > 0 {
		C.kcRead(env, req, n, unsafe.Pointer(&in[0]))
	}
	st := C.jint(statusOK)
	out, err := kcabi.Call(kcabi.Op(op), in)
	if err != nil {
		st = statusError
		var e *kcabi.Error
		if errors.As(err, &e) && e.Panic {
			st = statusPanic
		}
		out = []byte(err.Error())
	}
	C.kcStatus(env, status, st)
	var src unsafe.Pointer
	if len(out) > 0 {
		src = unsafe.Pointer(&out[0])
	}
	return C.kcNew(env, src, C.jsize(len(out)))
}

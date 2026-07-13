//go:build android

// Package binary is bound into libgojni.so by `gomobile bind -target=android`.
package binary

/*
#include <errno.h>

// gomobile compiles the Go "linux" build of modernc.org/libc for
// GOOS=android (Android satisfies the "linux" build constraint), which
// expects glibc's __errno_location() to fetch the per-thread errno.
//
// Android's bionic libc does NOT provide __errno_location(); it exposes
// __errno() instead. As a result the generated libgojni.so carries an
// undefined reference to __errno_location and the JVM fails to load it:
//
//	java.lang.UnsatisfiedLinkError: dlopen failed: cannot locate symbol
//	"__errno_location" referenced by ".../lib/arm64/libgojni.so"
//
// This shim defines __errno_location() in terms of bionic's __errno(),
// satisfying the reference so the shared library loads on Android.
int* __errno_location(void) {
	return __errno();
}
*/
import "C"

// ensureErrnoShimLinked keeps the C shim symbol referenced from Go so the
// linker does not drop it from libgojni.so.
func ensureErrnoShimLinked() {
	_ = C.__errno_location
}

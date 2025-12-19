package native

/*
#cgo CFLAGS: -O2 -Wall -Wextra -std=c11
#cgo pkg-config: openssl
#include "tls_tracker.c"
*/
import "C"

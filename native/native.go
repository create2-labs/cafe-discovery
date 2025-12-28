package native

/*
#cgo CFLAGS: -O2 -Wall -Wextra -std=c11 -I${SRCDIR}
#cgo pkg-config: openssl
#include <stdlib.h>
#include <stdbool.h>
#include "tls_pqc_wrapper.h"
*/
import "C"
import (
	"fmt"
	"unsafe"
)

// GetPQCInfo calls the C function get_pqc_info and returns the JSON string
func GetPQCInfo(host, port, group string, trace bool) (string, error) {
	cHost := C.CString(host)
	cPort := C.CString(port)
	cGroup := C.CString(group)
	cTrace := C.bool(trace)

	defer C.free(unsafe.Pointer(cHost))
	defer C.free(unsafe.Pointer(cPort))
	defer C.free(unsafe.Pointer(cGroup))

	cJSON := C.get_pqc_info(cHost, cPort, cGroup, cTrace)
	if cJSON == nil {
		return "", fmt.Errorf("get_pqc_info returned NULL")
	}
	defer C.free(unsafe.Pointer(cJSON))

	return C.GoString(cJSON), nil
}

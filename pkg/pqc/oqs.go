package pqc

/*
#cgo CFLAGS: -I${SRCDIR}/../native -I/opt/homebrew/opt/openssl@3/include -I/opt/liboqs/include
#cgo LDFLAGS: -L/opt/homebrew/opt/openssl@3/lib -L/opt/liboqs/lib -lssl -lcrypto -loqs -Wl,-rpath,/opt/liboqs/lib
#include <stdlib.h>
#include <string.h>
#include "../../native/oqs_wrapper.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"unsafe"
)

// MLDSA wraps liboqs ML-DSA signature functionality
type MLDSA struct {
	alg   string
	ctx   []byte
	sig   *C.OQS_SIG
	pkLen int
	skLen int
	sLen  int

	pk []byte
	sk []byte
}

// NewMLDSA creates a new ML-DSA signer with the specified algorithm and context
func NewMLDSA(alg string, ctx []byte) (*MLDSA, error) {
	cAlg := C.CString(alg)
	defer C.free(unsafe.Pointer(cAlg))

	s := C.go_oqs_sig_new(cAlg)
	if s == nil {
		// help debugging: list available alg identifiers
		var avail []string
		n := int(C.go_oqs_sig_algs_length())
		for i := 0; i < n; i++ {
			id := C.go_oqs_sig_alg_identifier(C.size_t(i))
			if id != nil {
				avail = append(avail, C.GoString(id))
			}
		}
		return nil, fmt.Errorf("OQS_SIG_new(%q) returned NULL (maybe disabled at build). Available: %v", alg, avail)
	}

	o := &MLDSA{
		alg:   alg,
		ctx:   append([]byte(nil), ctx...),
		sig:   s,
		pkLen: int(C.go_oqs_sig_pk_len(s)),
		skLen: int(C.go_oqs_sig_sk_len(s)),
		sLen:  int(C.go_oqs_sig_sig_len(s)),
	}

	o.pk = make([]byte, o.pkLen)
	o.sk = make([]byte, o.skLen)

	if st := C.go_oqs_sig_keypair(o.sig,
		(*C.uint8_t)(unsafe.Pointer(&o.pk[0])),
		(*C.uint8_t)(unsafe.Pointer(&o.sk[0])),
	); st != C.OQS_SUCCESS {
		o.Close()
		return nil, fmt.Errorf("keypair failed")
	}

	return o, nil
}

// Close releases resources associated with the signer
func (o *MLDSA) Close() {
	if o.sig != nil {
		C.go_oqs_sig_free(o.sig)
		o.sig = nil
	}
}

// PublicKey returns the public key
func (o *MLDSA) PublicKey() []byte {
	return append([]byte(nil), o.pk...)
}

// SetPublicKey sets the public key (for verification only)
func (o *MLDSA) SetPublicKey(pk []byte) error {
	if len(pk) != o.pkLen {
		return fmt.Errorf("public key length mismatch: got %d, expected %d", len(pk), o.pkLen)
	}
	o.pk = append([]byte(nil), pk...)
	return nil
}

// Sign signs a message
func (o *MLDSA) Sign(msg []byte) ([]byte, error) {
	if o.sig == nil {
		return nil, errors.New("oqs signer closed")
	}

	sig := make([]byte, o.sLen)
	sigLen := C.size_t(len(sig))

	var ctxPtr *C.uint8_t
	var ctxLen C.size_t
	if len(o.ctx) > 0 {
		ctxPtr = (*C.uint8_t)(unsafe.Pointer(&o.ctx[0]))
		ctxLen = C.size_t(len(o.ctx))
	}

	st := C.go_oqs_sig_sign_any(
		o.sig,
		(*C.uint8_t)(unsafe.Pointer(&sig[0])),
		&sigLen,
		(*C.uint8_t)(unsafe.Pointer(&msg[0])), C.size_t(len(msg)),
		ctxPtr, ctxLen,
		(*C.uint8_t)(unsafe.Pointer(&o.sk[0])),
	)
	if st != C.OQS_SUCCESS {
		return nil, fmt.Errorf("mldsa sign failed")
	}
	return sig[:int(sigLen)], nil
}

// Verify verifies a signature
func (o *MLDSA) Verify(msg, sig []byte) (bool, error) {
	if o.sig == nil {
		return false, errors.New("oqs signer closed")
	}
	if len(sig) == 0 || len(msg) == 0 {
		return false, nil
	}

	var ctxPtr *C.uint8_t
	var ctxLen C.size_t
	if len(o.ctx) > 0 {
		ctxPtr = (*C.uint8_t)(unsafe.Pointer(&o.ctx[0]))
		ctxLen = C.size_t(len(o.ctx))
	}

	st := C.go_oqs_sig_verify_any(
		o.sig,
		(*C.uint8_t)(unsafe.Pointer(&msg[0])), C.size_t(len(msg)),
		(*C.uint8_t)(unsafe.Pointer(&sig[0])), C.size_t(len(sig)),
		ctxPtr, ctxLen,
		(*C.uint8_t)(unsafe.Pointer(&o.pk[0])),
	)
	if st == C.OQS_SUCCESS {
		return true, nil
	}
	return false, nil
}

package pqc

/*
#cgo CFLAGS: -I${SRCDIR} -I${SRCDIR}/../native -I/opt/homebrew/opt/openssl@3/include -I/opt/liboqs/include
#cgo LDFLAGS: -L/opt/homebrew/opt/openssl@3/lib -L/opt/liboqs/lib -lssl -lcrypto -loqs -Wl,-rpath,/opt/liboqs/lib
#include <stdlib.h>
#include <string.h>
#include "oqs_wrapper.h"
// Note: oqs_wrapper.c is compiled by CGO in this package (shared with oqs.go)
*/
import "C"
import (
	"errors"
	"fmt"
	"unsafe"
)

// MLKEM wraps liboqs ML-KEM (Key Encapsulation Mechanism) functionality
type MLKEM struct {
	alg           string
	kem           *C.OQS_KEM
	pkLen         int
	skLen         int
	ciphertextLen int
	sharedLen     int

	pk []byte
	sk []byte
}

// NewMLKEM creates a new ML-KEM instance with the specified algorithm
func NewMLKEM(alg string) (*MLKEM, error) {
	cAlg := C.CString(alg)
	defer C.free(unsafe.Pointer(cAlg))

	k := C.go_oqs_kem_new(cAlg)
	if k == nil {
		return nil, fmt.Errorf("OQS_KEM_new(%q) returned NULL (maybe disabled at build)", alg)
	}

	o := &MLKEM{
		alg:           alg,
		kem:           k,
		pkLen:         int(C.go_oqs_kem_pk_len(k)),
		skLen:         int(C.go_oqs_kem_sk_len(k)),
		ciphertextLen: int(C.go_oqs_kem_ciphertext_len(k)),
		sharedLen:     int(C.go_oqs_kem_shared_secret_len(k)),
	}

	o.pk = make([]byte, o.pkLen)
	o.sk = make([]byte, o.skLen)

	if st := C.go_oqs_kem_keypair(o.kem,
		(*C.uint8_t)(unsafe.Pointer(&o.pk[0])),
		(*C.uint8_t)(unsafe.Pointer(&o.sk[0])),
	); st != C.OQS_SUCCESS {
		o.Close()
		return nil, fmt.Errorf("keypair failed")
	}

	return o, nil
}

// Close releases resources associated with the KEM
func (o *MLKEM) Close() {
	if o.kem != nil {
		C.go_oqs_kem_free(o.kem)
		o.kem = nil
	}
}

// PublicKey returns the public key
func (o *MLKEM) PublicKey() []byte {
	return append([]byte(nil), o.pk...)
}

// SetPublicKey sets the public key (for encapsulation only)
func (o *MLKEM) SetPublicKey(pk []byte) error {
	if len(pk) != o.pkLen {
		return fmt.Errorf("public key length mismatch: got %d, expected %d", len(pk), o.pkLen)
	}
	o.pk = append([]byte(nil), pk...)
	return nil
}

// Encapsulate generates a ciphertext and shared secret from the public key
func (o *MLKEM) Encapsulate(pk []byte) (ciphertext, sharedSecret []byte, err error) {
	if o.kem == nil {
		return nil, nil, errors.New("oqs kem closed")
	}
	if len(pk) != o.pkLen {
		return nil, nil, fmt.Errorf("public key length mismatch: got %d, expected %d", len(pk), o.pkLen)
	}

	ciphertext = make([]byte, o.ciphertextLen)
	sharedSecret = make([]byte, o.sharedLen)

	st := C.go_oqs_kem_encaps(
		o.kem,
		(*C.uint8_t)(unsafe.Pointer(&ciphertext[0])),
		(*C.uint8_t)(unsafe.Pointer(&sharedSecret[0])),
		(*C.uint8_t)(unsafe.Pointer(&pk[0])),
	)
	if st != C.OQS_SUCCESS {
		return nil, nil, fmt.Errorf("encapsulation failed")
	}

	return ciphertext, sharedSecret, nil
}

// Decapsulate recovers the shared secret from a ciphertext using the secret key
func (o *MLKEM) Decapsulate(ciphertext []byte) ([]byte, error) {
	if o.kem == nil {
		return nil, errors.New("oqs kem closed")
	}
	if len(ciphertext) != o.ciphertextLen {
		return nil, fmt.Errorf("ciphertext length mismatch: got %d, expected %d", len(ciphertext), o.ciphertextLen)
	}

	sharedSecret := make([]byte, o.sharedLen)

	st := C.go_oqs_kem_decaps(
		o.kem,
		(*C.uint8_t)(unsafe.Pointer(&sharedSecret[0])),
		(*C.uint8_t)(unsafe.Pointer(&ciphertext[0])),
		(*C.uint8_t)(unsafe.Pointer(&o.sk[0])),
	)
	if st != C.OQS_SUCCESS {
		return nil, fmt.Errorf("decapsulation failed")
	}

	return sharedSecret, nil
}

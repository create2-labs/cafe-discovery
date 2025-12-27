#ifndef OQS_WRAPPER_H
#define OQS_WRAPPER_H

#include <stddef.h>
#include <stdint.h>
#include <oqs/oqs.h>

// Wrapper functions for OQS_SIG to be used from Go via CGO
// These are needed because OQS_SIG uses function pointers which CGO cannot handle directly

OQS_SIG* go_oqs_sig_new(const char* alg);
void go_oqs_sig_free(OQS_SIG* s);

size_t go_oqs_sig_pk_len(OQS_SIG* s);
size_t go_oqs_sig_sk_len(OQS_SIG* s);
size_t go_oqs_sig_sig_len(OQS_SIG* s);
int go_oqs_sig_with_ctx(OQS_SIG* s);

OQS_STATUS go_oqs_sig_keypair(OQS_SIG* s, uint8_t* pk, uint8_t* sk);

OQS_STATUS go_oqs_sig_sign_any(OQS_SIG* s,
    uint8_t* sig, size_t* sig_len,
    const uint8_t* msg, size_t msg_len,
    const uint8_t* ctx, size_t ctx_len,
    const uint8_t* sk);

OQS_STATUS go_oqs_sig_verify_any(OQS_SIG* s,
    const uint8_t* msg, size_t msg_len,
    const uint8_t* sig, size_t sig_len,
    const uint8_t* ctx, size_t ctx_len,
    const uint8_t* pk);

// Enumerate algorithms (for debugging if ML-DSA-65 not enabled at build time)
size_t go_oqs_sig_algs_length(void);
const char* go_oqs_sig_alg_identifier(size_t i);

// Wrapper functions for OQS_KEM (Key Encapsulation Mechanism)
OQS_KEM* go_oqs_kem_new(const char* alg);
void go_oqs_kem_free(OQS_KEM* k);

size_t go_oqs_kem_pk_len(OQS_KEM* k);
size_t go_oqs_kem_sk_len(OQS_KEM* k);
size_t go_oqs_kem_ciphertext_len(OQS_KEM* k);
size_t go_oqs_kem_shared_secret_len(OQS_KEM* k);

OQS_STATUS go_oqs_kem_keypair(OQS_KEM* k, uint8_t* pk, uint8_t* sk);
OQS_STATUS go_oqs_kem_encaps(OQS_KEM* k, uint8_t* ciphertext, uint8_t* shared_secret, const uint8_t* pk);
OQS_STATUS go_oqs_kem_decaps(OQS_KEM* k, uint8_t* shared_secret, const uint8_t* ciphertext, const uint8_t* sk);

#endif // OQS_WRAPPER_H


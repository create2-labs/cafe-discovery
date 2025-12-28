#ifndef OQS_WRAPPER_C
#define OQS_WRAPPER_C

#include "oqs_wrapper.h"

OQS_SIG* go_oqs_sig_new(const char* alg) {
    return OQS_SIG_new(alg);
}

void go_oqs_sig_free(OQS_SIG* s) {
    OQS_SIG_free(s);
}

size_t go_oqs_sig_pk_len(OQS_SIG* s) {
    return s->length_public_key;
}

size_t go_oqs_sig_sk_len(OQS_SIG* s) {
    return s->length_secret_key;
}

size_t go_oqs_sig_sig_len(OQS_SIG* s) {
    return s->length_signature;
}

int go_oqs_sig_with_ctx(OQS_SIG* s) {
    return (s->sig_with_ctx_support && s->sign_with_ctx_str && s->verify_with_ctx_str) ? 1 : 0;
}

OQS_STATUS go_oqs_sig_keypair(OQS_SIG* s, uint8_t* pk, uint8_t* sk) {
    return s->keypair(pk, sk);
}

OQS_STATUS go_oqs_sig_sign_any(OQS_SIG* s,
    uint8_t* sig, size_t* sig_len,
    const uint8_t* msg, size_t msg_len,
    const uint8_t* ctx, size_t ctx_len,
    const uint8_t* sk)
{
    if (go_oqs_sig_with_ctx(s)) {
        return s->sign_with_ctx_str(sig, sig_len, msg, msg_len, ctx, ctx_len, sk);
    }
    (void)ctx; (void)ctx_len;
    return s->sign(sig, sig_len, msg, msg_len, sk);
}

OQS_STATUS go_oqs_sig_verify_any(OQS_SIG* s,
    const uint8_t* msg, size_t msg_len,
    const uint8_t* sig, size_t sig_len,
    const uint8_t* ctx, size_t ctx_len,
    const uint8_t* pk)
{
    if (go_oqs_sig_with_ctx(s)) {
        return s->verify_with_ctx_str(msg, msg_len, sig, sig_len, ctx, ctx_len, pk);
    }
    (void)ctx; (void)ctx_len;
    return s->verify(msg, msg_len, sig, sig_len, pk);
}

size_t go_oqs_sig_algs_length(void) {
    return OQS_SIG_algs_length;
}

const char* go_oqs_sig_alg_identifier(size_t i) {
    return OQS_SIG_alg_identifier(i);
}

// KEM wrapper functions
OQS_KEM* go_oqs_kem_new(const char* alg) {
    return OQS_KEM_new(alg);
}

void go_oqs_kem_free(OQS_KEM* k) {
    OQS_KEM_free(k);
}

size_t go_oqs_kem_pk_len(OQS_KEM* k) {
    return k->length_public_key;
}

size_t go_oqs_kem_sk_len(OQS_KEM* k) {
    return k->length_secret_key;
}

size_t go_oqs_kem_ciphertext_len(OQS_KEM* k) {
    return k->length_ciphertext;
}

size_t go_oqs_kem_shared_secret_len(OQS_KEM* k) {
    return k->length_shared_secret;
}

OQS_STATUS go_oqs_kem_keypair(OQS_KEM* k, uint8_t* pk, uint8_t* sk) {
    return k->keypair(pk, sk);
}

OQS_STATUS go_oqs_kem_encaps(OQS_KEM* k, uint8_t* ciphertext, uint8_t* shared_secret, const uint8_t* pk) {
    return k->encaps(ciphertext, shared_secret, pk);
}

OQS_STATUS go_oqs_kem_decaps(OQS_KEM* k, uint8_t* shared_secret, const uint8_t* ciphertext, const uint8_t* sk) {
    return k->decaps(shared_secret, ciphertext, sk);
}

#endif // OQS_WRAPPER_C


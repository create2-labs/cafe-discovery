// tls-tracker.c
// OpenSSL 3.x compatible
// Build:
//   cc -O2 -Wall tls-tracker.c -o tls-tracker -DTEST_MAIN \
//      -I/opt/homebrew/opt/openssl@3/include \
//      -L/opt/homebrew/opt/openssl@3/lib -lssl -lcrypto

#define _GNU_SOURCE
#include <ctype.h>
#include <stdarg.h>
#include <stdbool.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>

#include <openssl/asn1.h>
#include <openssl/bio.h>
#include <openssl/core_names.h>
#include <openssl/err.h>
#include <openssl/evp.h>
#include <openssl/objects.h>
#include <openssl/ssl.h>
#include <openssl/x509.h>
#include <openssl/x509v3.h>

/* ========== tiny json builder ========== */
typedef struct {
  char *buf;
  size_t len, cap;
} jbuf_t;
static void jb_init(jbuf_t *jb) {
  jb->buf = NULL;
  jb->len = jb->cap = 0;
}
static void jb_reserve(jbuf_t *jb, size_t add) {
  if (jb->len + add + 1 <= jb->cap)
    return;
  size_t n = jb->cap ? jb->cap : 256;
  while (jb->len + add + 1 > n)
    n *= 2;
  jb->buf = (char *)realloc(jb->buf, n);
  jb->cap = n;
}
static void jb_appendf(jbuf_t *jb, const char *fmt, ...) {
  va_list ap;
  va_start(ap, fmt);
  char tmp[2048];
  int n = vsnprintf(tmp, sizeof(tmp), fmt, ap);
  va_end(ap);
  if (n <= 0)
    return;
  jb_reserve(jb, (size_t)n);
  memcpy(jb->buf + jb->len, tmp, (size_t)n);
  jb->len += (size_t)n;
  jb->buf[jb->len] = '\0';
}
static char *json_escape(const char *s) {
  if (!s)
    return strdup("null");
  jbuf_t jb;
  jb_init(&jb);
  for (const unsigned char *p = (const unsigned char *)s; *p; ++p) {
    unsigned char c = *p;
    switch (c) {
    case '\"':
      jb_appendf(&jb, "\\\"");
      break;
    case '\\':
      jb_appendf(&jb, "\\\\");
      break;
    case '\n':
      jb_appendf(&jb, "\\n");
      break;
    case '\r':
      jb_appendf(&jb, "\\r");
      break;
    case '\t':
      jb_appendf(&jb, "\\t");
      break;
    default:
      if (c < 0x20)
        jb_appendf(&jb, "\\u%04x", c);
      else {
        jb_reserve(&jb, 1);
        jb.buf[jb.len++] = c;
        jb.buf[jb.len] = '\0';
      }
    }
  }
  return jb.buf;
}

/* ========== helpers X509/time ========== */
static void asn1_to_str(const ASN1_TIME *t, char out[64]) {
  if (!t) {
    snprintf(out, 64, "");
    return;
  }
  BIO *b = BIO_new(BIO_s_mem());
  ASN1_TIME_print(b, t);
  int n = BIO_read(b, out, 63);
  if (n > 0)
    out[n] = '\0';
  else
    out[0] = '\0';
  BIO_free(b);
}
static void sigalg_sn(const X509 *crt, char *out, size_t sz) {
  const X509_ALGOR *alg = X509_get0_tbs_sigalg(crt);
  if (!alg) {
    snprintf(out, sz, "");
    return;
  }
  const ASN1_OBJECT *aobj = NULL;
  X509_ALGOR_get0(&aobj, NULL, NULL, alg);
  int nid = OBJ_obj2nid(aobj);
  const char *sn = OBJ_nid2sn(nid);
  snprintf(out, sz, "%s", sn ? sn : "unknown");
}
static void pubkey_info(EVP_PKEY *pkey, char *type_out, size_t type_sz,
                        int *bits_out, char *ec_group, size_t ec_sz) {
  if (!pkey) {
    snprintf(type_out, type_sz, "");
    *bits_out = 0;
    if (ec_group)
      ec_group[0] = '\0';
    return;
  }
  int base = EVP_PKEY_base_id(pkey);
  *bits_out = EVP_PKEY_bits(pkey);
  if (base == EVP_PKEY_RSA)
    snprintf(type_out, type_sz, "rsa");
  else if (base == EVP_PKEY_EC)
    snprintf(type_out, type_sz, "ec");
  else if (base == EVP_PKEY_ED25519)
    snprintf(type_out, type_sz, "ed25519");
  else if (base == EVP_PKEY_ED448)
    snprintf(type_out, type_sz, "ed448");
  else
    snprintf(type_out, type_sz, "unknown(%d)", base);

  if (ec_group && base == EVP_PKEY_EC) {
    size_t n = 0;
    if (EVP_PKEY_get_utf8_string_param(pkey, OSSL_PKEY_PARAM_GROUP_NAME,
                                       ec_group, ec_sz, &n) != 1) {
      ec_group[0] = '\0'; // fallback unknown
    }
  } else if (ec_group)
    ec_group[0] = '\0';
}

/* ========== mappings per your rules ========== */

/* Cipher AEAD -> NIST level */
static int level_from_cipher(const char *cipher) {
  if (!cipher)
    return 0;
  if (strstr(cipher, "TLS_AES_256_GCM_SHA384"))
    return 5; // L5
  if (strstr(cipher, "TLS_AES_128_GCM_SHA256"))
    return 1; // L1
  if (strstr(cipher, "TLS_CHACHA20_POLY1305_SHA256"))
    return 1; // L1
  if (strstr(cipher, "TLS_AES_128_CCM"))
    return 1; // L1 (incl. _8_)
  // TLS 1.2 names (best effort)
  if (strstr(cipher, "AES256"))
    return 5;
  if (strstr(cipher, "AES128") || strstr(cipher, "CHACHA20"))
    return 1;
  return 0;
}

/* HKDF hash (TLS 1.3) -> NIST level */
static int level_from_hkdf(const char *cipher, const char *tlsv) {
  if (!tlsv || strncmp(tlsv, "TLSv1.3", 7) != 0)
    return 0;
  if (!cipher)
    return 0;
  if (strstr(cipher, "SHA384"))
    return 3;
  if (strstr(cipher, "SHA256"))
    return 1;
  return 0;
}

/* KEX group name (classique / PQC / hybride) -> NIST level */
static int level_from_kex_group(const char *group, const char *tlsv) {
  if (!tlsv || strncmp(tlsv, "TLSv1.3", 7) != 0)
    return 0;
  if (!group || !*group)
    return 0;

  int lvl = 0;

  // Classiques (ECDHE)
  if (strstr(group, "X25519") || strstr(group, "x25519") ||
      strstr(group, "secp256") || strstr(group, "P-256"))
    lvl = lvl > 1 ? lvl : 1;
  if (strstr(group, "secp384") || strstr(group, "P-384"))
    lvl = lvl > 3 ? lvl : 3;
  if (strstr(group, "secp521") || strstr(group, "P-521"))
    lvl = lvl > 5 ? lvl : 5;

  // PQC ML-KEM
  if (strstr(group, "MLKEM512") || strstr(group, "mlkem512"))
    lvl = lvl > 1 ? lvl : 1;
  if (strstr(group, "MLKEM768") || strstr(group, "mlkem768"))
    lvl = lvl > 3 ? lvl : 3;
  if (strstr(group, "MLKEM1024") || strstr(group, "mlkem1024"))
    lvl = lvl > 5 ? lvl : 5;

  // PQC Frodo
  if (strstr(group, "frodo640"))
    lvl = lvl > 1 ? lvl : 1;
  if (strstr(group, "frodo976"))
    lvl = lvl > 3 ? lvl : 3;
  if (strstr(group, "frodo1344"))
    lvl = lvl > 5 ? lvl : 5;

  // PQC BIKE (si jamais mappé)
  if (strstr(group, "bikel1"))
    lvl = lvl > 1 ? lvl : 1;
  if (strstr(group, "bikel3"))
    lvl = lvl > 3 ? lvl : 3;
  if (strstr(group, "bikel5"))
    lvl = lvl > 5 ? lvl : 5;

  return lvl;
}

/* Cert auth -> NIST level (pubkey/curve or PQC sig name) */
static int level_from_cert_auth(const char *pk_type, int pk_bits,
                                const char *ec_group, const char *sig_sn) {
  // PQC signatures ML-DSA (via sig OID short name)
  if (sig_sn && *sig_sn) {
    if (strstr(sig_sn, "ml-dsa-44"))
      return 2;
    if (strstr(sig_sn, "ml-dsa-65"))
      return 3;
    if (strstr(sig_sn, "ml-dsa-87"))
      return 5;
  }

  // EC: se baser sur le groupe si connu ; à défaut sur bits
  if (pk_type && strcmp(pk_type, "ec") == 0) {
    if (ec_group && *ec_group) {
      if (strstr(ec_group, "P-256") || strstr(ec_group, "prime256v1") ||
          strstr(ec_group, "secp256r1"))
        return 1;
      if (strstr(ec_group, "P-384") || strstr(ec_group, "secp384r1"))
        return 3;
      if (strstr(ec_group, "P-521") || strstr(ec_group, "secp521r1"))
        return 5;
    }
    if (pk_bits <= 256)
      return 1;
    else if (pk_bits <= 384)
      return 3;
    else
      return 5;
  }

  // EdDSA
  if (pk_type && strcmp(pk_type, "ed25519") == 0)
    return 1;
  if (pk_type && strcmp(pk_type, "ed448") == 0)
    return 3;

  // RSA thresholds
  if (pk_type && strcmp(pk_type, "rsa") == 0) {
    if (pk_bits >= 15360)
      return 5;
    if (pk_bits >= 7680)
      return 3;
    if (pk_bits >= 3072)
      return 1;
    return 0;
  }

  // Unknown → try by bits
  if (pk_bits >= 521)
    return 5;
  if (pk_bits >= 384)
    return 3;
  if (pk_bits >= 256)
    return 1;
  return 0;
}

/* ========== PQC mode detection (classical / hybrid / pure) ========== */
static const char *detect_pqc_mode(const char *group, const char *tlsv) {
  // En TLS < 1.3 → forcément classical
  if (!tlsv || strncmp(tlsv, "TLSv1.3", 7) != 0)
    return "classical";

  if (!group || !*group)
    return "classical";

  bool has_classical = false;
  bool has_pqc = false;

  /* Classical KEX (ECDHE, FFDHE, brainpool…) */
  if (strstr(group, "X25519") || strstr(group, "x25519"))
    has_classical = true;
  if (strstr(group, "X448") || strstr(group, "x448"))
    has_classical = true; // ← AJOUT
  if (strstr(group, "secp256") || strstr(group, "P-256"))
    has_classical = true;
  if (strstr(group, "secp384") || strstr(group, "P-384"))
    has_classical = true;
  if (strstr(group, "secp521") || strstr(group, "P-521"))
    has_classical = true;
  if (strstr(group, "brainpool"))
    has_classical = true;
  if (strstr(group, "bp256") || strstr(group, "bp384") ||
      strstr(group, "bp512")) // ← AJOUT
    has_classical = true;
  if (strstr(group, "ffdhe"))
    has_classical = true;

  /* PQC KEM (Kyber/ML-KEM, Frodo, BIKE…) */
  if (strstr(group, "MLKEM") || strstr(group, "mlkem"))
    has_pqc = true;
  if (strstr(group, "frodo"))
    has_pqc = true;
  if (strstr(group, "bike") || strstr(group, "BIKE") || strstr(group, "bikel"))
    has_pqc = true;

  /* Hybrid logic */
  if (has_classical && has_pqc)
    return "hybrid";
  if (has_pqc && !has_classical)
    return "pure";
  return "classical";
}

/* ========== TLS dial & info ========== */
static SSL *dial(const char *host, const char *port, const char *grp,
                 SSL_CTX **out_ctx, BIO **out_bio, char *err, size_t esz,
                 bool trace) {
  *out_ctx = NULL;
  *out_bio = NULL;
  ERR_clear_error();
  SSL_CTX *ctx = SSL_CTX_new(TLS_client_method());
  if (!ctx) {
    snprintf(err, esz, "SSL_CTX_new failed");
    return NULL;
  }
  SSL *ssl = SSL_new(ctx);
  if (!ssl) {
    snprintf(err, esz, "SSL_new failed");
    SSL_CTX_free(ctx);
    return NULL;
  }

  if (grp && *grp)
    SSL_set1_groups_list(ssl, grp);
  SSL_set_tlsext_host_name(ssl, host);

  char target[256];
  snprintf(target, sizeof(target), "%s:%s", host, port);
  BIO *bio = BIO_new_ssl_connect(ctx);
  BIO_get_ssl(bio, &ssl);
  SSL_set_mode(ssl, SSL_MODE_AUTO_RETRY);
  BIO_set_conn_hostname(bio, target);

  if (BIO_do_connect(bio) <= 0) {
    unsigned long e = ERR_get_error();
    char em[128];
    ERR_error_string_n(e, em, 128);
    snprintf(err, esz, "connect: %s", em);
    BIO_free_all(bio);
    SSL_CTX_free(ctx);
    return NULL;
  }
  *out_ctx = ctx;
  *out_bio = bio;
  if (trace)
    fprintf(stderr, "✅ Connected %s\n", target);
  return ssl;
}

/* ========== Public API ========== */
/**
 * Get connection informations for a given host and port
 * @param host: host
 * @param port: port
 * @param grp: group name (may be NULL)
 * @param trace: trace mode
 * @return JSON string (malloc'd, caller must free)
 */
char *get_pqc_info(const char *host, const char *port, const char *grp,
                   bool trace) {

  SSL_CTX *ctx = NULL;
  BIO *bio = NULL;
  char err[256] = "";
  jbuf_t jb;
  jb_init(&jb);

  SSL *ssl = dial(host, port, (grp && *grp) ? grp : NULL, &ctx, &bio, err,
                  sizeof(err), trace);
  if (!ssl) {
    char *he = json_escape(host), *ee = json_escape(err);
    jb_appendf(&jb, "{\"host\":\"%s\",\"error\":\"%s\"}", he, ee);
    free(he);
    free(ee);
    return jb.buf;
  }

  const char *tlsv = SSL_get_version(ssl);
  const SSL_CIPHER *sc = SSL_get_current_cipher(ssl);
  const char *cipher = sc ? SSL_CIPHER_get_name(sc) : "unknown";

  // negotiated group (classiques). For hybrids, fallback to grp (requested)
  char group[128] = "";
#if OPENSSL_VERSION_NUMBER >= 0x30000000L
  int nid = SSL_get_shared_group(ssl, 0);
  if (nid > 0) {
    const char *sn = OBJ_nid2sn(nid);
    if (sn)
      snprintf(group, sizeof(group), "%s", sn);
  }
#endif
  if (!group[0] && grp && *grp)
    snprintf(group, sizeof(group), "%s", grp);

  // Cert data
  X509 *crt = SSL_get1_peer_certificate(ssl);
  char subject[512] = "", issuer[512] = "", not_before[64] = "",
       not_after[64] = "", sig_sn[128] = "";
  char pk_type[64] = "", ec_group[64] = "";
  int pk_bits = 0;
  bool cert_expired = false;

  if (crt) {
    X509_NAME_oneline(X509_get_subject_name(crt), subject,
                      (int)sizeof(subject) - 1);
    X509_NAME_oneline(X509_get_issuer_name(crt), issuer,
                      (int)sizeof(issuer) - 1);
    asn1_to_str(X509_get0_notBefore(crt), not_before);
    asn1_to_str(X509_get0_notAfter(crt), not_after);
    sigalg_sn(crt, sig_sn, sizeof(sig_sn));

    EVP_PKEY *pkey = X509_get_pubkey(crt);
    if (pkey) {
      pubkey_info(pkey, pk_type, sizeof(pk_type), &pk_bits, ec_group,
                  sizeof(ec_group));
      EVP_PKEY_free(pkey);
    }
    time_t now = time(NULL);
    if (X509_cmp_time(X509_get0_notAfter(crt), &now) < 0)
      cert_expired = true;
  }

  /* ---- Levels per-component ---- */
  int lvl_kex = level_from_kex_group(group[0] ? group : NULL, tlsv);
  int lvl_cipher = level_from_cipher(cipher);
  int lvl_hkdf = level_from_hkdf(cipher, tlsv);
  int lvl_sig = level_from_cert_auth(pk_type, pk_bits, ec_group, sig_sn);

  // If TLS < 1.3 -> session level "none/not PQC": set 0
  int lvl_session = 0;
  if (tlsv && strncmp(tlsv, "TLSv1.3", 7) == 0) {
    // Session level = max of components (rule of thumb)
    lvl_session = lvl_kex;
    if (lvl_cipher > lvl_session)
      lvl_session = lvl_cipher;
    if (lvl_hkdf > lvl_session)
      lvl_session = lvl_hkdf;
    if (lvl_sig > lvl_session)
      lvl_session = lvl_sig;
  }

  // PQC bool: true si KEX implique un KEM PQC
  bool pqc = false;
  if (lvl_kex >= 1 && group[0]) {
    if (strstr(group, "MLKEM") || strstr(group, "mlkem") ||
        strstr(group, "frodo") || strstr(group, "bike") ||
        strstr(group, "BIKE") || strstr(group, "bikel")) {
      pqc = true;
    }
  }

  const char *pqc_mode = detect_pqc_mode(group[0] ? group : NULL, tlsv);

  /* ---- JSON ---- */
  char *h = json_escape(host), *p = json_escape(port),
       *tv = json_escape(tlsv ? tlsv : "unknown"),
       *gr = json_escape(group[0] ? group : "none"),
       *cy = json_escape(cipher ? cipher : "unknown"),
       *sub = json_escape(subject), *iss = json_escape(issuer),
       *nb = json_escape(not_before), *na = json_escape(not_after),
       *sig = json_escape(sig_sn), *pkt = json_escape(pk_type),
       *ecg = json_escape(ec_group);

  jb_appendf(&jb, "{");
  jb_appendf(&jb, "\"host\":\"%s\",\"port\":\"%s\",", h, p);
  jb_appendf(&jb, "\"tls_version\":\"%s\",\"group\":\"%s\",", tv, gr);
  jb_appendf(&jb, "\"pqc\":%s,", pqc ? "true" : "false");
  jb_appendf(&jb, "\"pqc_mode\":\"%s\",", pqc_mode);
  jb_appendf(&jb, "\"cipher_suite\":\"%s\",", cy);
  jb_appendf(&jb, "\"cert_subject\":\"%s\",\"cert_issuer\":\"%s\",", sub, iss);
  jb_appendf(&jb, "\"cert_not_before\":\"%s\",\"cert_not_after\":\"%s\",", nb,
             na);
  jb_appendf(&jb, "\"cert_expired\":%s,", cert_expired ? "true" : "false");
  jb_appendf(&jb,
             "\"cert_sig_alg\":\"%s\",\"cert_pubkey_type\":\"%s\",\"cert_"
             "pubkey_bits\":%d,",
             sig, pkt, pk_bits);
  jb_appendf(&jb, "\"cert_pubkey_ec_group\":\"%s\",", ecg);

  jb_appendf(&jb, "\"nist_levels\":{");
  jb_appendf(&jb,
             "\"kex\":%d,\"sig\":%d,\"cipher\":%d,\"hkdf\":%d,\"session\":%d",
             lvl_kex, lvl_sig, lvl_cipher, lvl_hkdf, lvl_session);
  jb_appendf(&jb, "}");

  jb_appendf(&jb, "}");

  free(h);
  free(p);
  free(tv);
  free(gr);
  free(cy);
  free(sub);
  free(iss);
  free(nb);
  free(na);
  free(sig);
  free(pkt);
  free(ecg);
  if (crt)
    X509_free(crt);
  BIO_free_all(bio);
  SSL_CTX_free(ctx);

  return jb.buf; // caller must free()
}

/* ========== simple CLI (TEST_MAIN) ========== */
#ifdef TEST_MAIN
int main(int argc, char **argv) {
  if (argc < 2) {
    fprintf(stderr, "Usage: %s host[:port] [--group name] [--trace]\n",
            argv[0]);
    return 1;
  }
  const char *grp = NULL;
  bool trace = false;
  for (int i = 2; i < argc; i++) {
    if (!strcmp(argv[i], "--trace"))
      trace = true;
    else if (!strcmp(argv[i], "--group") && i + 1 < argc)
      grp = argv[++i];
  }

  // Parse host[:port]
  char host[128] = "", port[16] = "443";
  const char *host_in = argv[1];
  const char *c = strchr(host_in, ':');
  if (c) {
    size_t n = (size_t)(c - host_in);
    if (n >= sizeof(host))
      n = sizeof(host) - 1;
    memcpy(host, host_in, n);
    host[n] = '\0';
    snprintf(port, sizeof(port), "%s", c + 1);
  } else {
    snprintf(host, sizeof(host), "%s", host_in);
  }

  char *json = get_pqc_info(host, port, grp, trace);
  puts(json);
  free(json);
  return 0;
}
#endif

# OpenSSL and OQS provider

---

## TL;DR

```
docker build --no-cache -t openssl-oqs -f Dockerfile-oqs-slim .
docker run -it --rm openssl-oqs  bash
```

Dans le container tester avec 
```
$> openssl list -providers     

Providers:
  default
    name: OpenSSL Default Provider
    version: 3.0.17
    status: active
  oqsprovider
    name: OpenSSL OQS Provider
    version: 0.10.1-dev
    status: active

$>  openssl list -kem-algorithms
  { 1.2.840.113549.1.1.1, 2.5.8.1.1, RSA, rsaEncryption } @ default
  frodo640aes @ oqsprovider
  p256_frodo640aes @ oqsprovider
  x25519_frodo640aes @ oqsprovider
  frodo640shake @ oqsprovider
  p256_frodo640shake @ oqsprovider
  x25519_frodo640shake @ oqsprovider
  frodo976aes @ oqsprovider
  p384_frodo976aes @ oqsprovider
  x448_frodo976aes @ oqsprovider
  frodo976shake @ oqsprovider
  p384_frodo976shake @ oqsprovider
  x448_frodo976shake @ oqsprovider
  frodo1344aes @ oqsprovider
  p521_frodo1344aes @ oqsprovider
  frodo1344shake @ oqsprovider
  p521_frodo1344shake @ oqsprovider
  mlkem512 @ oqsprovider
  p256_mlkem512 @ oqsprovider
  x25519_mlkem512 @ oqsprovider
  bp256_mlkem512 @ oqsprovider
  mlkem768 @ oqsprovider
  p384_mlkem768 @ oqsprovider
  x448_mlkem768 @ oqsprovider
  bp384_mlkem768 @ oqsprovider
  X25519MLKEM768 @ oqsprovider
  SecP256r1MLKEM768 @ oqsprovider
  mlkem1024 @ oqsprovider
  p521_mlkem1024 @ oqsprovider
  SecP384r1MLKEM1024 @ oqsprovider
  bp512_mlkem1024 @ oqsprovider
  bikel1 @ oqsprovider
  p256_bikel1 @ oqsprovider
  x25519_bikel1 @ oqsprovider
  bikel3 @ oqsprovider
  p384_bikel3 @ oqsprovider
  x448_bikel3 @ oqsprovider
  bikel5 @ oqsprovider
  p521_bikel5 @ oqsprovider
```

* Generate ML-KEM key

```
openssl genpkey -algorithm ML-KEM-512 -out mlkem.key
```

* Encapsulate/decapsulate

```
openssl pkeyutl -inkey mlkem.key -kem -capsuleout capsule.bin
openssl pkeyutl -inkey mlkem.key -kem -capsulein capsule.bin -secretout secret.bin
```

* Generate ML-DSA signature

```
openssl genpkey -algorithm ML-DSA-65 -out mldsa.key
openssl pkey -in mldsa.key -pubout > mldsa.pub
echo "hello" | openssl dgst -sigopt skey:mldsa.key -sign mldsa.key -out sig.bin
```


---

## Introduction

Transport Layer Security (TLS) is the foundation of secure Internet communications.
It protects confidentiality and integrity across HTTPS, APIs, and VPNs.
However, current TLS implementations rely on **RSA** and **Elliptic-Curve cryptography**,
which are **vulnerable to quantum attacks**.

This guide walks you through:

* How TLS and key exchange groups work
* Why quantum computers threaten current cryptography
* How the **hybrid post-quantum mode** mitigates the risk
* How to compile and use **OQS (Open Quantum Safe)** components with OpenSSL 3
* And how to run your own hybrid TLS server and client

---

## TLS in a Nutshell

A **TLS handshake** negotiates an encrypted channel between a client and a server.

```
ClientHello →  Supported Groups, Cipher Suites
ServerHello →  Chosen Group, Cipher
Certificate →  Server proves its identity
Key Exchange → Shared secret derived
Finished → Encrypted channel established
```

### Main steps

1. **Key exchange** – client and server derive a shared secret (ECDH or RSA-based).
2. **Certificate verification** – server proves its identity with a public key signature.
3. **Symmetric encryption** – all traffic is encrypted using a derived AES or ChaCha20 key.

---

## Key Exchange Groups

In TLS 1.3, *groups* define how ephemeral keys are generated:

| Group     | Type | Bits    | Example        |
| --------- | ---- | ------- | -------------- |
| X25519    | ECDH | 128-bit | Modern default |
| secp256r1 | ECDH | 128-bit | Legacy         |
| X448      | ECDH | 224-bit | High-security  |

The client advertises supported groups; the server chooses one.
This determines the cryptographic base of the session.

---

## The Quantum Threat

Quantum computers running **Shor’s algorithm** can solve the discrete log and factoring problems in polynomial time.

| Classical Algorithm | Broken by Shor? | Status                              |
| ------------------- | --------------- | ----------------------------------- |
| RSA                 | ✅               | Insecure under quantum              |
| ECDH / ECDSA        | ✅               | Insecure under quantum              |
| AES-256             | ❌               | Still safe (Grover halves security) |

Meaning: once a large-enough quantum computer exists, any TLS handshake recorded today could be decrypted tomorrow — the **“harvest now, decrypt later”** scenario.

---

## Post-Quantum Cryptography (PQC)

To counter this, new algorithms are being standardized by NIST:

| Category                 | NIST Standard     | Purpose            |
| ------------------------ | ----------------- | ------------------ |
| **ML-KEM** (Kyber)       | Key Encapsulation | Key exchange       |
| **ML-DSA** (Dilithium)   | Digital Signature | Authentication     |
| **FALCON**, **SPHINCS+** | Alternatives      | Digital signatures |

But TLS integration is still experimental — IETF drafts and vendors implement pre-standard “hybrid” modes.

---

## The Hybrid Mode (Classical + PQC)

A **hybrid key exchange** combines a classical ECDH and a PQC KEM:

```
Client and Server perform both:
    - Classical ECDH (e.g., X25519)
    - Post-quantum KEM (e.g., ML-KEM-768)

Final session key = HKDF( secret_ECDH || secret_PQC )
```

Thus, the session remains secure even if:

* The classical algorithm is broken by quantum computers, or
* The PQC algorithm turns out to be flawed.

Typical hybrid group names:

* `X25519MLKEM768`
* `secp384r1MLKEM1024`

### Hybrid handshake flow

```
+---------+                               +---------+
| Client  |                               | Server  |
+---------+                               +---------+
| 1. Offer classical + PQC groups ----------->      |
|                                                   |
| <-------- 2. Choose hybrid group (X25519MLKEM768) |
| 3. Send ECDH + PQC pubkeys ---------------------> |
| <------------------ 4. Send ECDH + PQC response   |
| 5. Derive combined secret                         |
| 6. Verify certificate (ML-DSA signature)          |
+---------------------------------------------------+
```

---

## Providers in OpenSSL 3+

### Concept

Since version 3, OpenSSL uses **modular providers** — dynamic libraries that expose cryptographic algorithms.
Each provider registers algorithms in categories: cipher, digest, signature, KEM, etc.

| Provider        | Role                                           |
| --------------- | ---------------------------------------------- |
| **default**     | Classical OpenSSL algorithms (RSA, AES, SHA2…) |
| **legacy**      | Deprecated (MD5, RC4…)                         |
| **oqsprovider** | PQC & hybrid algorithms via liboqs             |
| **fips**        | Certified FIPS-validated module                |

### Architecture Overview

```
+------------------------------------------------+
|                   OpenSSL 3.x                  |
|------------------------------------------------|
|  Applications / CLI / Libraries (EVP API)      |
|------------------------------------------------|
|       ↳ loads providers dynamically            |
|------------------------------------------------|
|  [default]  [legacy]  [oqsprovider]  [fips]    |
|     |           |          |          |        |
|  Classical   Old algos   PQC algos   Compliance|
+------------------------------------------------+
                     ↑
                     |
        liboqs (C library implementing ML-KEM, ML-DSA, etc.)
```


### Configuration file (`openssl.cnf`)

Located at:

* macOS (Homebrew): `/opt/homebrew/etc/openssl@3/openssl.cnf`
* Linux: `/etc/ssl/openssl.cnf`

Add at the end:

```ini
openssl_conf = openssl_init

[openssl_init]
providers = provider_sect

[provider_sect]
default = default_sect
oqsprovider = oqs_sect

[default_sect]
activate = 1

[oqs_sect]
activate = 1
module = /usr/local/lib/ossl-modules/oqsprovider.dylib
```

---

## The Open Quantum Safe Project (OQS)

OQS provides the building blocks for quantum-safe cryptography:

| Component       | Description                                                         |
| --------------- | ------------------------------------------------------------------- |
| **liboqs**      | Core C library implementing PQC KEMs & signatures                   |
| **oqsprovider** | OpenSSL 3 provider loading liboqs algorithms into the EVP framework |

These two layers integrate seamlessly with OpenSSL — giving immediate access to hybrid TLS without patching OpenSSL itself.

---

## Architecture Diagram

```
                 +-----------------------+
                 |  Application / TLS    |
                 +----------+------------+
                            |
                 +----------v------------+
                 |     OpenSSL 3.6 CLI   |
                 |  (EVP / TLS / X.509)  |
                 +----------+------------+
                            |
      +---------------------+---------------------+
      |        Provider Interface (OSSL)          |
      +----------+------------+-------------------+
                 |            |
     +-----------v--+   +-----v------------------+
     | default.so   |   | oqsprovider.so         |
     | (RSA, ECDH)  |   | (ML-KEM, ML-DSA, etc.) |
     +-------+------+   +-----------+------------+
             |                      |
      +------v------+        +------v------+
      | libcrypto   |        | liboqs      |
      | (OpenSSL)   |        | PQC backend |
      +-------------+        +-------------+
```

---

## Building liboqs and oqsprovider

* [MacOS (Apple Silicon / Intel)](./install_oqs_openssl_mac.sh)
* [Linux (Ubuntu/Debian)](./install_oqs_openssl_debian.sh)
* [Docker](./Dockerfile-oqs-slim)

---

## Verifying Your Installation

```bash
openssl list -providers
openssl list -public-key-algorithms -provider oqsprovider | grep mlkem
```

Expected:

```
Providers:
  default
  oqsprovider
p256_mlkem512
p384_mlkem768
p521_mlkem1024
x25519_mlkem768
```

```
√ build % strings /opt/homebrew/opt/openssl@3/lib/ossl-modules/oqsprovider.dylib | egrep -i 'MLKEM|Kyber|X25519MLKEM|SecP256r1MLKEM' | head

mlkem512
mlkem768
X25519MLKEM768
SecP256r1MLKEM768
SecP384r1MLKEM1024
mlkem1024
p256_mlkem512
x25519_mlkem512
bp256_mlkem512
p384_mlkem768
√ build % /opt/homebrew/opt/openssl@3/bin/openssl list -key-exchange-algorithms -provider oqsprovider -provider default                    

  { 1.2.840.113549.1.3.1, DH, dhKeyAgreement } @ default
  { 1.3.101.110, X25519 } @ default
  { 1.3.101.111, X448 } @ default
  ECDH @ default
  TLS1-PRF @ default
  HKDF @ default
  { 1.3.6.1.4.1.11591.4.11, id-scrypt, SCRYPT } @ default
√ build % /opt/homebrew/opt/openssl@3/bin/openssl list -kem-algorithms -provider oqsprovider -provider default

  SecP384r1MLKEM1024 @ default
  { 1.2.840.10045.2.1, EC, id-ecPublicKey } @ default
  { 1.3.101.110, X25519 } @ default
  { 1.3.101.111, X448 } @ default
  { 2.16.840.1.101.3.4.4.1, id-alg-ml-kem-512, ML-KEM-512, MLKEM512 } @ default
  { 2.16.840.1.101.3.4.4.2, id-alg-ml-kem-768, ML-KEM-768, MLKEM768 } @ default
  { 2.16.840.1.101.3.4.4.3, id-alg-ml-kem-1024, ML-KEM-1024, MLKEM1024 } @ default
  X25519MLKEM768 @ default
  X448MLKEM1024 @ default
  SecP256r1MLKEM768 @ default
  { 1.2.840.113549.1.1.1, 2.5.8.1.1, RSA, rsaEncryption } @ default
  x448_frodo976shake @ oqsprovider
  frodo1344aes @ oqsprovider
  p521_frodo1344aes @ oqsprovider
  frodo1344shake @ oqsprovider
  p521_frodo1344shake @ oqsprovider
  p256_mlkem512 @ oqsprovider
  x25519_mlkem512 @ oqsprovider
  bp256_mlkem512 @ oqsprovider
  p384_mlkem768 @ oqsprovider
  x448_mlkem768 @ oqsprovider
  bp384_mlkem768 @ oqsprovider
  p521_mlkem1024 @ oqsprovider
  bp512_mlkem1024 @ oqsprovider
  bikel1 @ oqsprovider
  p256_bikel1 @ oqsprovider
  frodo640aes @ oqsprovider
  bikel3 @ oqsprovider
  p384_bikel3 @ oqsprovider
  x448_bikel3 @ oqsprovider
  bikel5 @ oqsprovider
  p521_bikel5 @ oqsprovider
  x25519_bikel1 @ oqsprovider
  p256_frodo640aes @ oqsprovider
  x25519_frodo640aes @ oqsprovider
  frodo640shake @ oqsprovider
  p256_frodo640shake @ oqsprovider
  x25519_frodo640shake @ oqsprovider
  frodo976aes @ oqsprovider
  p384_frodo976aes @ oqsprovider
  x448_frodo976aes @ oqsprovider
  frodo976shake @ oqsprovider
  p384_frodo976shake @ oqsprovider
```

```
cat /opt/homebrew/etc/openssl@3/openssl-oqs.cnf
openssl_conf = openssl_init

[openssl_init]
providers = provider_sect

[provider_sect]
default = default_sect
oqsprovider = oqs_sect

[default_sect]
activate = 1

[oqs_sect]
activate = 1
module = /opt/homebrew/opt/openssl@3/lib/ossl-modules/oqsprovider.dylib

```

---

## Testing Post-Quantum TLS Locally

```bash
# Generate hybrid private key
/opt/homebrew/opt/openssl@3/bin/openssl/openssl genpkey -provider default -provider oqsprovider \
  -algorithm p384_mldsa65 -out p384_mldsa65_priv.pem

# Create CSR
/opt/homebrew/opt/openssl@3/bin/openssl/openssl req -new -provider default -provider oqsprovider \
  -key p384_mldsa65_priv.pem -out p384_mldsa65.csr \
  -subj "/C=FR/ST=Ile-de-France/L=Paris/O=CAFE/OU=Quantum/CN=localhost"

# Self-sign
/opt/homebrew/opt/openssl@3/bin/openssl/openssl x509 -req -provider default -provider oqsprovider \
  -in p384_mldsa65.csr -signkey p384_mldsa65_priv.pem \
  -out p384_mldsa65_cert.pem -days 365 -sha512

# Run server
/opt/homebrew/opt/openssl@3/bin/openssl/openssl s_server -provider default -provider oqsprovider \
  -cert p384_mldsa65_cert.pem -key p384_mldsa65_priv.pem -port 8443

# Connect client
/opt/homebrew/opt/openssl@3/bin/openssl/openssl s_client -provider default -provider oqsprovider -connect localhost:8443
```

You should see:

```
Peer signature type: p384_mldsa65
Negotiated TLS1.3 group: X25519MLKEM768
Cipher: TLS_AES_256_GCM_SHA384
Verification error: self-signed certificate
```

That’s a fully operational **hybrid post-quantum TLS 1.3 handshake**.

---

## 1Detecting PQC Support on Remote Endpoints

```bash
./pq-scan https://test.openquantumsafe.org
```

Output:

```json
{
  "cipher_suite": "TLS_AES_256_GCM_SHA384",
  "kex_alg": "X25519MLKEM768",
  "cert_sig_alg": "p384_mldsa65",
  "pqc_ready": true,
  "score": 100
}
```

---

## 16. Conclusion

Quantum computers will one day render classical TLS obsolete.
With OQS and OpenSSL 3 providers, you can already test, verify, and deploy hybrid post-quantum TLS today — preparing your systems for crypto-agility and quantum-readiness.

---

## Annexe - some OpenSSL tips and tricks 

### `BIO` — OpenSSL input/output abstraction

`BIO` (Basic I/O abstraction) is the base of all communication in OpenSSL.
Its encapsulates data flows : TCP socket, file, memory etc.

OpenSSL does not directly use sockets (`read()` / `write()`) :
it uses a `BIO`, permiting to stack layers (compression, crypto, bufferisation...).


---

### `cipher` — cryptographic suite


A cipher suite is a set of primitives:

* KEX (Key Exchange) : ECDHE, X25519, ML-KEM, etc.
* Authentification : RSA, ECDSA…
* Symetric crypto : AES-GCM, ChaCha20-Poly1305…
* Hachage / integrity : SHA256, SHA384, etc.

Example :

```
TLS_AES_256_GCM_SHA384
```

→ means :

* TLS 1.3
* AES-256 cryptography with GCM mode
* SHA-384 integrity


### `EVP` — higher cryptographic layer

OpenSSL dinstinguishes TLS logic and pure cryptography
`EVP` (EnVeloPe) is a high level generic API opermiting to abstract algorithms.

Examples :

* `EVP_PKEY` → contains a key pair (RSA, EC, ED25519…)
* `EVP_CIPHER` → represents a symetric algorithm (AES, CHACHA…)
* `EVP_MD` → represents a hash function (SHA256…)


### `SAN` — *Subject Alternative Name*

SAN are X.509 extensions listing all valid DNS entries for a given certificate.

Example :

```
Subject: CN = test.openquantumsafe.org
SANs: DNS:test.openquantumsafe.org; DNS:www.openquantumsafe.org;
```

### `has_ocsp` — OCSP stapling (Online Certificate Status Protocol)

OCSP is a protocol that permits to verify certificate revocation.
Rather than ask a certificate authority, server may *“staple”* a valid OCSP response in the TLS handshake.


### `ALPN` — *Application-Layer Protocol Negotiation*

ALPN is a TMLS extension that eases the choice of the applicative protocol between server and client.

Examples :

* `h2` : HTTP/2
* `http/1.1` : HTTP 1
* `acme-tls/1` : Let’s Encrypt ACME protocol
* `mqtt`, `imap`, etc.


---

## Summary

| Concept  | Type            | TLS role                            | Example                  |
| -------- | --------------- | ----------------------------------- | ------------------------ |
| `BIO`    | I/O abstraction | SSL socket abstraction              | `BIO_new_connect()`      |
| `cipher` | TLS parameter   | Cryptographic suite                 | `TLS_AES_256_GCM_SHA384` |
| `EVP`    | Crypto API      | Keys and algo generic abstraction   | `EVP_PKEY`, `EVP_CIPHER` |
| `SAN`    | X.509 extension | Certificate DNS names               | `DNS:www.exemple.com`    |
| `OCSP`   | X.509 status    | Efficiant cert revocation check     | Stapled OCSP response    |
| `ALPN`   | TLS extension   | Applicativ protocol negociation     | `h2`, `http/1.1`         |

---

```

                           ┌──────────────────────────────────────────────┐
                           │                  Application                │
                           │                (ton code C)                 │
                           └──────────────────────────────────────────────┘
                                             │
                                             ▼
                     ┌──────────────────────────────────────────────┐
                     │                 SSL object                   │
                     │        (TLS state machine per session)       │
                     │──────────────────────────────────────────────│
                     │ manages:                                     │
                     │  • handshake TLS (ClientHello, ServerHello)  │
                     │  • cipher suite negotiation                  │
                     │  • key exchange (ECDHE, MLKEM, etc.)         │
                     │  • encryption/decryption of data             │
                     │  • ALPN, OCSP, extensions…                   │
                     └──────────────────────────────────────────────┘
                                             ▲
        SSL_set_bio(ssl, bio, bio)           │
           │                                 │
           │ uses                            │ reads/writes
           ▼                                 │
   ┌──────────────────────────┐              │
   │          BIO             │──────────────┘
   │ (Basic I/O abstraction)  │
   │──────────────────────────│
   │ wraps:                   │
   │  • TCP sockets (BIO_s_connect)          ←──  BIO_new_connect("host:port")
   │  • files (BIO_s_file)                   ←──  BIO_new_file("cert.pem", "r")
   │  • memory buffers (BIO_s_mem)           ←──  BIO_new(BIO_s_mem())
   └──────────────────────────┘
           │
           ▼
   ┌──────────────────────────┐
   │        TCP socket        │
   │──────────────────────────│
   │ OS-level connection to   │
   │ test.openquantumsafe.org │
   │ port 443 / 6002          │
   └──────────────────────────┘
           │
           ▼
   ┌──────────────────────────┐
   │        SSL_CTX           │
   │──────────────────────────│
   │ Global config shared by  │
   │ multiple SSL sessions:   │
   │  • allowed TLS versions  │
   │  • cipher suites         │
   │  • PQC groups (MLKEM…)   │ ←── SSL_CTX_set1_groups_list()
   │  • trusted CA store      │
   │  • providers (OQS)       │ ←── OSSL_PROVIDER_load("oqsprovider")
   └──────────────────────────┘
           │
           ▼
   ┌──────────────────────────┐
   │        Handshake         │
   │──────────────────────────│
   │ 1. ClientHello           │
   │    → proposes ciphers, groups (ex: SecP384r1MLKEM1024)         │
   │ 2. ServerHello           │
   │    → picks one + sends cert                                   │
   │ 3. Key exchange (ECDHE/PQC)                                   │
   │ 4. Finished messages (MAC check)                              │
   └──────────────────────────┘
           │
           ▼
   ┌──────────────────────────┐
   │    X.509 Certificate     │
   │──────────────────────────│
   │ Subject: CN=test.openqsafe.org          ←── X509_get_subject_name()      │
   │ Issuer:  CN=oqstest_intermediate_ecdsap256  ←── X509_get_issuer_name()   │
   │ Validity: notBefore / notAfter          ←── ASN1_TIME_print()            │
   │ Signature Algorithm: ecdsa-with-SHA256  ←── X509_get_signature_nid()     │
   │ Public Key: EC 256 bits                 ←── EVP_PKEY_get_pubkey()        │
   │ SANs: DNS:test.openquantumsafe.org;     ←── X509_get_ext_d2i(..., NID_subject_alt_name) │
   │ OCSP Stapling: yes/no                   ←── SSL_get0_ocsp_resp()         │
   └──────────────────────────┘
           │
           ▼
   ┌──────────────────────────┐
   │      Cipher Suite        │
   │──────────────────────────│
   │ TLS_AES_256_GCM_SHA384   ←── SSL_get_current_cipher()                    │
   │ KEX: SecP384r1MLKEM1024  ←── SSL_get_negotiated_group() + heuristics     │
   │ PQC ready: true           │
   └──────────────────────────┘
           │
           ▼
   ┌──────────────────────────┐
   │         EVP Layer        │
   │──────────────────────────│
   │ Unified crypto interface │
   │  • EVP_PKEY = public key │
   │  • EVP_CIPHER = AES-GCM  │
   │  • EVP_MD = SHA-384      │
   │ abstracts algorithms via │
   │ providers (default + oqs)│
   └──────────────────────────┘
           │
           ▼
   ┌──────────────────────────┐
   │          JSON            │
   │──────────────────────────│
   │ {                        │
   │   "tls_version":"TLS1.3",│
   │   "cipher":"TLS_AES_256_GCM_SHA384",   │
   │   "kex_group":"SecP384r1MLKEM1024",   │
   │   "cert_subject":"/CN=test..."         │
   │ }                        │
   └──────────────────────────┘
```




# CAFE sniffer

CAFE sniffer is a local network sensor that measures in real time the NIST level of encryption used in TLS, SSH, QUIC, IPsec, etc. streams.

CAFE sniffer detects the NIST level of packets circulating on the local network.

CAFE sniffer could be integrated into CAFE Discovery: a page will display the NIST level of sniffed packets in real time.

---

# Architecture

```
          +--------------------------+
          | Local Sniffer C++       |
          |--------------------------|
          | + sniff packet          |
          | + detect protocol       |
          | + parse ClientHello     |
          | + extract suites        |
          | + map → NIST Level      |
          +--------------------------+
                     |
                     v
          +--------------------------+
          | REST / Kafka / stdout   |
          | JSON events             |
          +--------------------------+
                     |
                     v
          +--------------------------+
          | CAFE Discovery Dashboard |
          | Graphs, gauges, scoring |
          +--------------------------+
```

---

# NIST levels

| KEM         | NIST level |
| ----------- | ---------- |
| ML-KEM-512  | L1         |
| ML-KEM-768  | L3         |
| ML-KEM-1024 | L5         |
| Falcon-512  | L1         |
| Falcon-1024 | L5         |
| ML-DSA-44   | L1         |
| ML-DSA-87   | L3         |

---

# Build

- Debian / Ubuntu

```
sudo apt-get install libssl-dev libpcap-dev
```

- Mac OS

```
brew install openssl@3 libpcap
```

---

## Requirements

Next must be installed and configured on your PC

```
- openssl@3
- oqsprovider
```

Needed environment variables

```
- OPENSSL_MODULES
- OQS_PROVIDER
- OPENSSL_CONF
```

---

# Run

Here two tests cases. They permit to see what CAFE sniffer get

## Test classic TLS (non-PQC)

1. Generate RSA key

```
/opt/homebrew/opt/openssl@3/bin/openssl genrsa -out server.key 2048
/opt/homebrew/opt/openssl@3/bin/openssl req -new -x509 -key server.key -out server.crt -subj "/CN=localhost"
```

2. Start non  PQC server

```
/opt/homebrew/opt/openssl@3/bin/openssl s_server \
  -key server.key \
  -cert server.crt \
  -accept 8443 \
  -cipher DEFAULT \
  -www
```

3. CAFE sniffer should see

* `cipher_suites`: TLS_AES_xxx, CHACHA20…
* **no** hybrid group
* `nist_max_level = 0`
* `has_pqc = false`


4. Start non PQC TLS client

```
/opt/homebrew/opt/openssl@3/bin/openssl s_client \
  -connect localhost:8443
```

CAFE sniffer should detect a pre quantique ClientHello.


## Test hybrid PQC TLS (OpenSSL 3 + oqsprovider)

1. Generate PQC/Hybride key

We still use a classic X.509 cert; hybrid PQC relates to **KEM** only.

1. Start hybride PQC server

Example : **X25519 + ML-KEM-768**

```bash
/opt/homebrew/opt/openssl@3/bin/openssl s_server \
  -key server.key \
  -cert server.crt \
  -accept 8555 \
  -groups X25519MLKEM768 \
  -www \
  -provider default \
  -provider oqsprovider
```

This enforeces **X25519MLKEM768** hybrid KEM.

CAFE sniffer should display

```json
{
  "has_pqc": true,
  "has_hybrid": true,
  "nist_max_level": 3,
  "hybrids": [
    "X25519MLKEM768"
  ]
}
```

---

2. Start hybride PQC client

```bash
/opt/homebrew/opt/openssl@3/bin/openssl s_client \
  -connect localhost:8555 \
  -groups X25519MLKEM768 \
  -provider default \
  -provider oqsprovider
```

One can try

```
-groups X25519:MLKEM768
```

---

## Test different NIST levels

## ML-KEM-512 (NIST L1)

Server :

```bash
openssl s_server \
  -key server.key -cert server.crt \
  -accept 8666 \
  -groups P256MLKEM512 \
  -provider default -provider oqsprovider -www
```

Client :

```bash
openssl s_client \
  -connect localhost:8666 \
  -groups P256MLKEM512 \
  -provider default -provider oqsprovider
```

→ Sniffer : `nist_max_level = 1`

---

## ML-KEM-1024 (NIST L5)

Serveur :

```bash
openssl s_server \
  -key server.key -cert server.crt \
  -accept 8777 \
  -groups X25519MLKEM1024 \
  -provider default -provider oqsprovider -www
```

Client :

```bash
openssl s_client \
  -connect localhost:8777 \
  -groups X25519MLKEM1024 \
  -provider default -provider oqsprovider
```

→ Sniffer : `nist_max_level = 5`

---

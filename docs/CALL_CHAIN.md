# Chaîne d'appels - TLS Scan avec PQC

## Flux complet

```
HTTP Request
    ↓
internal/handler/tls.go::TLSHandler.Scan()
    ↓
internal/service/tls.go::TLSService.ScanTLS()
    ↓
pkg/tls/scanner.go::Scanner.ScanURL()
    ↓
pkg/tls/scanner.go::Scanner.Scan()  [fait un dial Go avec crypto/tls]
    ↓
pkg/tls/scanner.go::Scanner.scanPQCInfo()  [appelle le code C]
    ↓
pkg/tls/scanner.go::scanPQCInfo() → ScanPQC() pour chaque groupe
    ↓
pkg/tls/pqc_scanner.go::ScanPQC()
    ↓
native/native.go::GetPQCInfo()  [CGO bridge]
    ↓
native/tls_pqc/tls_pqc_scan.c::get_pqc_info()  [CODE C]
    ↓
native/tls_pqc/tls_pqc_scan.c::dial()  [CODE C - fait le dial()]
    ↓
OpenSSL BIO_do_connect()  [connexion TCP + TLS handshake]
```

## Détails

### 1. Handler (HTTP)
**Fichier**: `internal/handler/tls.go`
- `TLSHandler.Scan()` reçoit la requête HTTP
- Parse l'URL
- Appelle `TLSService.ScanTLS()`

### 2. Service (Business Logic)
**Fichier**: `internal/service/tls.go`
- `TLSService.ScanTLS()` orchestre le scan
- Appelle `scanner.ScanURL()`
- Calcule le risque et génère les recommandations

### 3. Scanner Go (crypto/tls)
**Fichier**: `pkg/tls/scanner.go`
- `Scanner.ScanURL()` parse l'URL
- `Scanner.Scan()` fait un **premier dial avec Go crypto/tls**:
  ```go
  conn, err := tls.DialWithDialer(dialer, "tcp", net.JoinHostPort(host, port), config)
  ```
  - Ce dial Go est utilisé pour:
    - Détecter la version TLS
    - Obtenir les certificats
    - Obtenir les cipher suites
- `Scanner.scanPQCInfo()` appelle ensuite le code C pour détecter PQC

### 4. Scanner PQC (CGO)
**Fichier**: `pkg/tls/pqc_scanner.go`
- `ScanPQC(host, port, group, trace)` appelle le code C
- Appelé pour chaque groupe dans `DefaultPQCGroups`:
  - D'abord sans groupe (scan initial)
  - Puis avec chaque groupe hybrid (X25519MLKEM768, X25519Kyber768Draft00, etc.)

### 5. Bridge CGO
**Fichier**: `native/native.go`
- `GetPQCInfo(host, port, group, trace)` est le bridge CGO
- Convertit les strings Go en C strings
- Appelle `get_pqc_info()` du code C
- Retourne le JSON string

### 6. Code C - API publique
**Fichier**: `native/tls_pqc/tls_pqc_scan.c`
- `get_pqc_info(host, port, grp, trace)` est l'entrée principale
- Appelle `dial()` pour établir la connexion
- Extrait les informations TLS/PQC
- Retourne un JSON string (malloc'd)

### 7. Code C - Dial
**Fichier**: `native/tls_pqc/tls_pqc_scan.c` (lignes 382-425)

**⚠️ PROBLÈME IDENTIFIÉ**: Le code actuel a un bug - la configuration (groupe, SNI) est perdue.

```c
static SSL *dial(...) {
  SSL_CTX *ctx = SSL_CTX_new(TLS_client_method());
  SSL *ssl = SSL_new(ctx);  // ← Crée un SSL
  
  // Configure le groupe et SNI sur ssl
  if (grp && *grp)
    SSL_set1_groups_list(ssl, grp);  // ← Configuration groupe
  SSL_set_tlsext_host_name(ssl, host);  // ← Configuration SNI
  
  BIO *bio = BIO_new_ssl_connect(ctx);  // ← Crée un NOUVEAU SSL interne
  BIO_get_ssl(bio, &ssl);  // ← ⚠️ PROBLÈME: Écrase ssl avec le SSL du BIO!
  // La configuration précédente est perdue!
  
  BIO_set_conn_hostname(bio, target);
  BIO_do_connect(bio);  // ← Connexion TCP seulement (pas handshake TLS)
  // Le handshake TLS se fait automatiquement lors de la première I/O
  return ssl;
}
```

**Problèmes**:
1. `BIO_new_ssl_connect(ctx)` crée un **nouveau SSL** interne
2. `BIO_get_ssl(bio, &ssl)` **écrase** le `ssl` configuré avec le SSL du BIO
3. La configuration (groupe hybrid, SNI) est **perdue**
4. `BIO_do_connect()` fait seulement la **connexion TCP**, pas le handshake TLS

**Correction nécessaire**:
- Soit utiliser `BIO_set_ssl()` pour attacher le SSL configuré
- Soit configurer le SSL **après** `BIO_get_ssl()`

## Points importants

1. **Deux dials séparés**:
   - **Dial Go** (`crypto/tls`): Pour obtenir les infos de base (version, certs, ciphers)
   - **Dial C** (`OpenSSL`): Pour détecter PQC avec groupes spécifiques

2. **Le code C fait son propre dial()**:
   - `BIO_do_connect()` dans `dial()` établit la **connexion TCP seulement**
   - Le **TLS handshake** se fait automatiquement lors de la première I/O sur le BIO SSL
   - C'est indépendant du dial Go

3. **⚠️ BUG dans le code actuel**:
   - La configuration (groupe hybrid, SNI) est **perdue** à cause de `BIO_get_ssl()`
   - `BIO_new_ssl_connect()` crée un nouveau SSL qui écrase celui configuré
   - **Correction nécessaire**: Configurer le SSL après `BIO_get_ssl()` ou utiliser `BIO_set_ssl()`

4. **Pourquoi deux dials?**:
   - Le dial Go est rapide et donne les infos de base
   - Le dial C est nécessaire pour tester des groupes PQC spécifiques
   - Le code C devrait demander explicitement des groupes hybrides via `SSL_set1_groups_list()`
   - **Mais actuellement cette config est perdue à cause du bug**

5. **Ordre des scans PQC**:
   - Scan initial sans groupe (serveur peut offrir PQC proactivement)
   - Si pas de PQC détecté, scan avec chaque groupe dans `DefaultPQCGroups`
   - Si handshake réussit avec un groupe hybrid → détection immédiate
   - **Mais le bug fait que le groupe n'est pas vraiment utilisé dans le handshake**

## Code C - dial() - Ligne 382-425 (VERSION ACTUELLE - AVEC BUG)

```c
static SSL *dial(const char *host, const char *port, const char *grp,
                 SSL_CTX **out_ctx, BIO **out_bio, char *err, size_t esz,
                 bool trace) {
  SSL_CTX *ctx = SSL_CTX_new(TLS_client_method());
  SSL *ssl = SSL_new(ctx);  // ← Crée un SSL
  
  // Configure le groupe si fourni (ex: "X25519MLKEM768")
  if (grp && *grp)
    SSL_set1_groups_list(ssl, grp);  // ← Configuration groupe
  
  // Configure le hostname pour SNI
  SSL_set_tlsext_host_name(ssl, host);  // ← Configuration SNI
  
  // Crée le BIO pour la connexion
  char target[256];
  snprintf(target, sizeof(target), "%s:%s", host, port);
  BIO *bio = BIO_new_ssl_connect(ctx);  // ← Crée un NOUVEAU SSL interne
  BIO_get_ssl(bio, &ssl);  // ← ⚠️ BUG: Écrase ssl avec le SSL du BIO!
  // La configuration (groupe, SNI) est PERDUE ici!
  
  SSL_set_mode(ssl, SSL_MODE_AUTO_RETRY);
  BIO_set_conn_hostname(bio, target);
  
  // ← ICI: Connexion TCP seulement (pas handshake TLS)
  if (BIO_do_connect(bio) <= 0) {
    // Erreur de connexion TCP
    return NULL;
  }
  // Le handshake TLS se fait automatiquement lors de la première I/O
  
  return ssl;  // ← Retourne le SSL du BIO (sans la config originale)
}
```

**⚠️ PROBLÈME**: La configuration est perdue car `BIO_get_ssl()` écrase le `ssl` configuré.

**CORRECTION PROPOSÉE**:
```c
// Option 1: Configurer APRÈS BIO_get_ssl()
BIO *bio = BIO_new_ssl_connect(ctx);
BIO_get_ssl(bio, &ssl);  // Récupère le SSL du BIO
if (grp && *grp)
  SSL_set1_groups_list(ssl, grp);  // ← Configurer APRÈS
SSL_set_tlsext_host_name(ssl, host);  // ← Configurer APRÈS

// Option 2: Utiliser BIO_set_ssl() (si le BIO le supporte)
// Mais BIO_new_ssl_connect() crée déjà un SSL, donc pas applicable ici
```

## Résumé

- **Qui appelle le code C?**: `native.GetPQCInfo()` (via CGO) appelé depuis `pkg/tls/pqc_scanner.go::ScanPQC()`
- **Qui fait le dial()?**: Le code C dans `native/tls_pqc/tls_pqc_scan.c::dial()` via `BIO_do_connect()`
- **Pourquoi deux dials?**: Dial Go pour infos de base, dial C pour tester groupes PQC spécifiques

# Génération de Certificats Post-Quantum Cryptography (PQC)

Ce guide explique comment générer des certificats TLS utilisant des algorithmes post-quantiques pour tester le scanner TLS.

## Prérequis

### Option 1 : oqs-provider avec OpenSSL 3.x (Recommandé - Plus simple)

`oqs-provider` est un provider OpenSSL 3 qui ajoute le support des algorithmes post-quantiques. C'est la méthode la plus simple et moderne.

**Référence**: [https://github.com/open-quantum-safe/oqs-provider](https://github.com/open-quantum-safe/oqs-provider)

#### Installation d'OpenSSL 3.x

```bash
# Sur Ubuntu/Debian
sudo apt-get update
sudo apt-get install openssl libssl-dev

# Vérifier la version (doit être 3.0+)
openssl version
```

#### Installation d'oqs-provider

```bash
# Cloner le dépôt
git clone https://github.com/open-quantum-safe/oqs-provider.git
cd oqs-provider

# Installer les dépendances (liboqs sera compilé automatiquement)
mkdir _build && cd _build
cmake ..
make -j$(nproc)
sudo make install

# Configurer OpenSSL pour utiliser le provider
# Ajouter dans ~/.bashrc ou ~/.zshrc
export OPENSSL_CONF=/usr/local/ssl/openssl.cnf
# Ou créer un openssl.cnf local (voir ci-dessous)
```

#### Configuration d'OpenSSL pour utiliser oqs-provider

Créer un fichier `openssl-pqc.cnf` :

```ini
openssl_conf = openssl_init

[openssl_init]
providers = provider_sect

[provider_sect]
default = default_sect
oqsprovider = oqsprovider_sect

[default_sect]
activate = 1

[oqsprovider_sect]
activate = 1
```

Utiliser ce fichier :
```bash
export OPENSSL_CONF=/chemin/vers/openssl-pqc.cnf
# Ou utiliser -config lors des commandes
openssl req -config openssl-pqc.cnf ...
```

#### Vérifier l'installation

```bash
# Lister les algorithmes PQC disponibles
openssl list -provider oqsprovider -public-key-algorithms | grep -E "(dilithium|falcon)"

# Devrait afficher: dilithium2, dilithium3, dilithium5, falcon512, falcon1024, etc.
```

### Option 2 : OpenSSL standard (pour Ed25519)

Ed25519 est résistant aux attaques quantiques et supporté par OpenSSL standard :

```bash
openssl version
# Version 1.1.1 ou supérieure recommandée
```

### Option 3 : OpenSSL avec Open Quantum Safe complet (Ancienne méthode)

Si vous préférez compiler OpenSSL-OQS complet (méthode plus complexe) :

```bash
# Installer OpenSSL-OQS
git clone https://github.com/open-quantum-safe/openssl.git
cd openssl
git submodule update --init
./Configure linux-x86_64 --with-oqs
make -j$(nproc)
sudo make install
```

## Génération de Certificats

### Méthode rapide avec le script

```bash
# Utiliser le script fourni (s'assurer que oqs-provider est configuré)
./scripts/generate-pqc-cert.sh dilithium3 365 localhost

# Ou avec des options personnalisées
./scripts/generate-pqc-cert.sh falcon512 365 test.example.com
```

### Méthode manuelle avec oqs-provider

**Important**: Assurez-vous que `oqs-provider` est installé et configuré (voir prérequis ci-dessus).

#### 1. Certificat avec Dilithium3 (NIST Standard)

```bash
# Avec le provider configuré globalement
openssl genpkey -algorithm dilithium3 -out dilithium3.key
openssl req -new -x509 -key dilithium3.key \
  -out dilithium3.crt -days 365 \
  -subj "/CN=localhost/O=Test PQC/C=FR"

# OU avec -config si vous utilisez un fichier de config local
openssl genpkey -config openssl-pqc.cnf \
  -algorithm dilithium3 -out dilithium3.key
openssl req -config openssl-pqc.cnf -new -x509 \
  -key dilithium3.key -out dilithium3.crt -days 365 \
  -subj "/CN=localhost/O=Test PQC/C=FR"
```

#### 2. Certificat avec Falcon1024 (NIST Standard)

```bash
openssl genpkey -algorithm falcon1024 -out falcon1024.key
openssl req -new -x509 -key falcon1024.key \
  -out falcon1024.crt -days 365 \
  -subj "/CN=localhost/O=Test PQC/C=FR"
```

#### 3. Certificat avec Ed25519 (Quantum-Resistant, largement supporté)

```bash
openssl genpkey -algorithm ED25519 -out ed25519.key
openssl req -new -x509 -key ed25519.key \
  -out ed25519.crt -days 365 \
  -subj "/CN=localhost/O=Test Quantum-Resistant/C=FR"
```

## Algorithmes PQC Disponibles

### Signatures numériques (NIST standardisés)

| Algorithme | Niveau NIST | Usage |
|------------|-------------|-------|
| `dilithium2` | 2 | Signatures, taille moyenne |
| `dilithium3` | 3 | Signatures, recommandé |
| `dilithium5` | 5 | Signatures, sécurité maximale |
| `falcon512` | 1 | Signatures, compact |
| `falcon1024` | 5 | Signatures, haute sécurité |

### Algorithmes quantum-resistant (non-PQC standard)

| Algorithme | Support | Usage |
|------------|---------|-------|
| `ED25519` | Large | Compatible, quantum-resistant |
| `ED448` | Moyen | Alternative à Ed25519 |

## Tester avec un serveur HTTPS local

### 1. Générer le certificat

```bash
./scripts/generate-pqc-cert.sh dilithium3 365 localhost
```

### 2. Lancer un serveur HTTPS avec Go

Créer un fichier `test-server.go` :

```go
package main

import (
    "crypto/tls"
    "fmt"
    "net/http"
)

func main() {
    mux := http.NewServeMux()
    mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintf(w, "Hello from PQC server!")
    })

    server := &http.Server{
        Addr:    ":8443",
        Handler: mux,
        TLSConfig: &tls.Config{
            MinVersion: tls.VersionTLS12,
        },
    }

    fmt.Println("Starting PQC server on :8443")
    err := server.ListenAndServeTLS("localhost-dilithium3.crt", "localhost-dilithium3.key")
    if err != nil {
        panic(err)
    }
}
```

### 3. Lancer le serveur

```bash
go run test-server.go
```

### 4. Scanner avec l'API

```bash
curl -X POST http://localhost:8080/discovery/scan/endpoints \
  -H "Content-Type: application/json" \
  -d '{"url": "https://localhost:8443"}' | jq
```

## Vérification du certificat

```bash
# Voir les détails du certificat
openssl x509 -in dilithium3.crt -text -noout

# Vérifier l'algorithme de signature
openssl x509 -in dilithium3.crt -text -noout | grep "Signature Algorithm"

# Vérifier l'algorithme de clé publique
openssl x509 -in dilithium3.crt -text -noout | grep "Public Key Algorithm"
```

## Limitations actuelles

⚠️ **Important** : Les certificats PQC ont des limitations :

1. **Support navigateur** : Les navigateurs ne supportent pas encore nativement les certificats PQC
2. **TLS 1.3** : Le support PQC dans TLS 1.3 est encore expérimental
3. **Autorités de certification** : Aucune CA publique ne délivre encore de certificats PQC
4. **Interopérabilité** : Peu de serveurs/clients supportent actuellement les certificats PQC

## Ressources

- **[oqs-provider](https://github.com/open-quantum-safe/oqs-provider)** - Provider OpenSSL 3 pour algorithmes PQC (Recommandé)
- [Open Quantum Safe](https://openquantumsafe.org/)
- [NIST PQC Standards](https://csrc.nist.gov/projects/post-quantum-cryptography)
- [OQS-OpenSSL Repository](https://github.com/open-quantum-safe/openssl) (méthode alternative)
- [Documentation oqs-provider](https://github.com/open-quantum-safe/oqs-provider/blob/main/USAGE.md)


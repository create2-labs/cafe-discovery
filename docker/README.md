# Docker pour génération de certificats PQC

Ce Dockerfile permet de créer un environnement isolé avec OpenSSL 3 et oqs-provider pour générer des certificats post-quantiques sans modifier votre système.

## Construction de l'image

```bash
# Rendre le script exécutable
chmod +x docker/build-pqc-openssl.sh

# Builder l'image
./docker/build-pqc-openssl.sh
```

Ou manuellement:

```bash
docker build -t pqc-openssl:latest -f docker/Dockerfile.pqc-openssl .
```

## Utilisation

### Méthode 1 : Interactive

```bash
# Lancer un shell interactif
docker run -it --rm \
  -v $(pwd):/certs \
  pqc-openssl:latest

# Dans le container, vous pouvez utiliser openssl directement
openssl list -provider oqsprovider -public-key-algorithms
```

### Méthode 2 : Avec le script helper

```bash
# Générer un certificat Dilithium3
docker run -it --rm \
  -v $(pwd):/certs \
  pqc-openssl:latest \
  generate-pqc-cert.sh dilithium3 365 localhost

# Générer un certificat Falcon1024
docker run -it --rm \
  -v $(pwd):/certs \
  pqc-openssl:latest \
  generate-pqc-cert.sh falcon1024 365 test.example.com
```

### Méthode 3 : Commandes OpenSSL directes

```bash
# Générer une clé privée (avec Falcon car disponible par défaut)
docker run -it --rm \
  -v $(pwd):/certs \
  pqc-openssl:latest \
  openssl genpkey -algorithm falcon512 -out /certs/server.key

# Générer un certificat
docker run -it --rm \
  -v $(pwd):/certs \
  pqc-openssl:latest \
  openssl req -new -x509 \
    -key /certs/server.key \
    -out /certs/server.crt \
    -days 365 \
    -subj "/CN=localhost/O=Test PQC/C=FR"
```

### Méthode 4 : Serveur de test HTTPS avec certificat PQC

Le container inclut deux serveurs de test :

#### Option A: Serveur OpenSSL avec certificat PQC (Recommandé pour tester PQC)

```bash
# 1. Générer un certificat PQC
docker run -it --rm \
  -v $(pwd)/certs:/certs \
  pqc-openssl:latest \
  generate-pqc-cert.sh falcon512 365 localhost

# 2. Démarrer le serveur OpenSSL avec le certificat PQC
docker run -d --rm \
  --name pqc-server \
  -p 8443:8443 \
  -v $(pwd)/certs:/certs \
  pqc-openssl:latest \
  start-pqc-server.sh /certs/localhost-falcon512.crt /certs/localhost-falcon512.key 8443

# 3. Tester le serveur (OpenSSL s_server répond avec des infos TLS)
curl -k https://localhost:8443/
# Affiche des informations sur la connexion TLS

# 4. Scanner avec votre API pour analyser le certificat PQC
curl -X POST http://localhost:8080/discovery/scan/endpoints \
  -H "Content-Type: application/json" \
  -d '{"url": "https://localhost:8443"}'

# 5. Arrêter
docker stop pqc-server
```

#### Option B: Serveur Go avec certificat RSA (pour tests classiques)

```bash
# Générer un certificat RSA standard (Go ne supporte pas PQC)
docker run -it --rm \
  -v $(pwd)/certs:/certs \
  pqc-openssl:latest \
  generate-standard-cert.sh /certs localhost

# Démarrer le serveur Go
docker run -d --rm \
  --name test-server \
  -p 8444:8444 \
  -v $(pwd)/certs:/certs \
  pqc-openssl:latest \
  start-test-server.sh /certs/localhost-rsa.crt /certs/localhost-rsa.key 8444

# Tester
curl -k https://localhost:8444/
# Réponse: ok
```

## Algorithmes disponibles

Lister les algorithmes PQC disponibles:

```bash
docker run -it --rm pqc-openssl:latest \
  openssl list -provider oqsprovider -public-key-algorithms | grep -E "(dilithium|falcon)"
```

Algorithmes courants:
- `dilithium2` - Dilithium niveau 2
- `dilithium3` - Dilithium niveau 3 (recommandé)
- `dilithium5` - Dilithium niveau 5
- `falcon512` - Falcon 512
- `falcon1024` - Falcon 1024

## Vérifier un certificat généré

```bash
docker run -it --rm \
  -v $(pwd):/certs \
  pqc-openssl:latest \
  openssl x509 -in /certs/localhost-dilithium3.crt -text -noout
```

## Exemple complet

```bash
# 1. Builder l'image
./docker/build-pqc-openssl.sh

# 2. Générer un certificat (avec Falcon car Dilithium peut ne pas être activé)
docker run -it --rm \
  -v $(pwd)/certs:/certs \
  pqc-openssl:latest \
  generate-pqc-cert.sh falcon512 365 localhost

# 3. Les fichiers seront dans ./certs/
ls -lh certs/
# localhost-falcon512.key
# localhost-falcon512.crt

# 4. Démarrer un serveur HTTPS avec certificat PQC (OpenSSL s_server)
docker run -d --rm \
  --name pqc-test-server \
  -p 8443:8443 \
  -v $(pwd)/certs:/certs \
  pqc-openssl:latest \
  start-pqc-server.sh /certs/localhost-falcon512.crt /certs/localhost-falcon512.key 8443

# 5. Tester le serveur (OpenSSL s_server affiche des infos TLS)
curl -k https://localhost:8443/
# Affiche des informations sur la connexion TLS avec certificat PQC

# 6. Scanner avec votre API
curl -X POST http://localhost:8080/discovery/scan/endpoints \
  -H "Content-Type: application/json" \
  -d '{"url": "https://localhost:8443"}'

# 7. Arrêter le serveur de test
docker stop pqc-test-server
```

## Notes

- Les certificats générés sont sauvegardés dans le répertoire monté (`-v $(pwd):/certs`)
- Le container est supprimé automatiquement après utilisation (`--rm`)
- L'image fait environ 500-800 MB (après compilation)
- Le build initial peut prendre 10-20 minutes (compilation de liboqs et oqs-provider)

## Troubleshooting

### Erreur: "provider oqsprovider not found"

Vérifier que le provider est bien installé:
```bash
docker run -it --rm pqc-openssl:latest \
  openssl list -providers
```

### Erreur: "algorithm X not available"

Vérifier les algorithmes disponibles:
```bash
docker run -it --rm pqc-openssl:latest \
  openssl list -provider oqsprovider -public-key-algorithms
```

### Optimiser la taille de l'image

Pour réduire la taille, vous pouvez créer une image multi-stage qui ne garde que les binaires nécessaires (non inclus dans ce Dockerfile pour la simplicité).


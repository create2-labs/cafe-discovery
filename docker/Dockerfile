# Dockerfile pour OpenSSL 3 avec oqs-provider
# Permet de générer des certificats post-quantiques

FROM ubuntu:22.04

ENV DEBIAN_FRONTEND=noninteractive
ENV OPENSSL_CONF=/etc/ssl/openssl-pqc.cnf

# Installer les dépendances nécessaires
RUN apt-get update && apt-get install -y \
    build-essential \
    cmake \
    git \
    ninja-build \
    libssl-dev \
    openssl \
    pkg-config \
    ca-certificates \
    golang-go \
    && rm -rf /var/lib/apt/lists/*

# Créer un répertoire de travail
WORKDIR /opt

# Cloner et compiler liboqs (dépendance de oqs-provider)
RUN git clone --depth 1 --branch main https://github.com/open-quantum-safe/liboqs.git && \
    cd liboqs && \
    mkdir build && cd build && \
    cmake -GNinja -DCMAKE_INSTALL_PREFIX=/usr/local .. && \
    ninja && \
    ninja install && \
    cd ../.. && \
    rm -rf liboqs

# Cloner et compiler oqs-provider
RUN git clone --depth 1 --branch main https://github.com/open-quantum-safe/oqs-provider.git && \
    cd oqs-provider && \
    mkdir _build && cd _build && \
    cmake -GNinja -DCMAKE_INSTALL_PREFIX=/usr/local .. && \
    ninja && \
    ninja install && \
    cd ../.. && \
    rm -rf oqs-provider

# Trouver le chemin du provider et créer un lien symbolique si nécessaire
RUN PROVIDER_PATH=$(find /usr/local -name "oqsprovider.so" 2>/dev/null | head -1) && \
    if [ -n "$PROVIDER_PATH" ]; then \
    echo "Provider trouvé: $PROVIDER_PATH"; \
    mkdir -p /usr/lib/ssl/ossl-modules; \
    ln -sf "$PROVIDER_PATH" /usr/lib/ssl/ossl-modules/oqsprovider.so 2>/dev/null || true; \
    fi && \
    PROVIDER_DIR=$(find /usr/local -type d -name "ossl-modules" 2>/dev/null | head -1) && \
    if [ -n "$PROVIDER_DIR" ]; then \
    echo "Module dir: $PROVIDER_DIR"; \
    fi

# Créer le fichier de configuration OpenSSL pour utiliser oqs-provider
RUN mkdir -p /etc/ssl && \
    echo "openssl_conf = openssl_init" > /etc/ssl/openssl-pqc.cnf && \
    echo "" >> /etc/ssl/openssl-pqc.cnf && \
    echo "[openssl_init]" >> /etc/ssl/openssl-pqc.cnf && \
    echo "providers = provider_sect" >> /etc/ssl/openssl-pqc.cnf && \
    echo "" >> /etc/ssl/openssl-pqc.cnf && \
    echo "[provider_sect]" >> /etc/ssl/openssl-pqc.cnf && \
    echo "default = default_sect" >> /etc/ssl/openssl-pqc.cnf && \
    echo "oqsprovider = oqsprovider_sect" >> /etc/ssl/openssl-pqc.cnf && \
    echo "" >> /etc/ssl/openssl-pqc.cnf && \
    echo "[default_sect]" >> /etc/ssl/openssl-pqc.cnf && \
    echo "activate = 1" >> /etc/ssl/openssl-pqc.cnf && \
    echo "" >> /etc/ssl/openssl-pqc.cnf && \
    echo "[oqsprovider_sect]" >> /etc/ssl/openssl-pqc.cnf && \
    echo "activate = 1" >> /etc/ssl/openssl-pqc.cnf

# Vérifier que tout fonctionne (|| true pour ne pas faire échouer le build si le provider n'est pas encore chargé)
RUN openssl version && \
    openssl list -providers || true && \
    (openssl list -provider oqsprovider -public-key-algorithms 2>/dev/null | head -20 || echo "Note: Provider sera disponible après configuration")

# Créer un répertoire de travail pour les certificats
WORKDIR /certs

# Script helper pour générer des certificats PQC
RUN echo '#!/bin/bash' > /usr/local/bin/generate-pqc-cert.sh && \
    echo '# Helper script pour générer des certificats PQC' >> /usr/local/bin/generate-pqc-cert.sh && \
    echo 'export OPENSSL_CONF=/etc/ssl/openssl-pqc.cnf' >> /usr/local/bin/generate-pqc-cert.sh && \
    echo 'ALGORITHM=${1:-falcon512}' >> /usr/local/bin/generate-pqc-cert.sh && \
    echo 'DAYS=${2:-365}' >> /usr/local/bin/generate-pqc-cert.sh && \
    echo 'CN=${3:-localhost}' >> /usr/local/bin/generate-pqc-cert.sh && \
    echo 'OUTPUT_DIR=${4:-/certs}' >> /usr/local/bin/generate-pqc-cert.sh && \
    echo '' >> /usr/local/bin/generate-pqc-cert.sh && \
    echo 'echo "Generation d'"'"'un certificat PQC avec:"' >> /usr/local/bin/generate-pqc-cert.sh && \
    echo 'echo "  Algorithme: $ALGORITHM"' >> /usr/local/bin/generate-pqc-cert.sh && \
    echo 'echo "  Durée: $DAYS jours"' >> /usr/local/bin/generate-pqc-cert.sh && \
    echo 'echo "  CN: $CN"' >> /usr/local/bin/generate-pqc-cert.sh && \
    echo 'echo ""' >> /usr/local/bin/generate-pqc-cert.sh && \
    echo '' >> /usr/local/bin/generate-pqc-cert.sh && \
    echo 'cd "$OUTPUT_DIR"' >> /usr/local/bin/generate-pqc-cert.sh && \
    echo '' >> /usr/local/bin/generate-pqc-cert.sh && \
    echo 'PROVIDER_DIR=$(find /usr/local -type d -name "ossl-modules" 2>/dev/null | head -1)' >> /usr/local/bin/generate-pqc-cert.sh && \
    echo 'if [ -z "$PROVIDER_DIR" ]; then' >> /usr/local/bin/generate-pqc-cert.sh && \
    echo '  PROVIDER_DIR="/usr/lib/ssl/ossl-modules"' >> /usr/local/bin/generate-pqc-cert.sh && \
    echo 'fi' >> /usr/local/bin/generate-pqc-cert.sh && \
    echo '' >> /usr/local/bin/generate-pqc-cert.sh && \
    echo 'if ! openssl list -provider-path "$PROVIDER_DIR" -provider oqsprovider -provider default -public-key-algorithms 2>/dev/null | grep -q "$ALGORITHM"; then' >> /usr/local/bin/generate-pqc-cert.sh && \
    echo '  echo "ERREUR: Algorithme $ALGORITHM non disponible"' >> /usr/local/bin/generate-pqc-cert.sh && \
    echo '  echo "Algorithmes disponibles:"' >> /usr/local/bin/generate-pqc-cert.sh && \
    echo '  openssl list -provider-path "$PROVIDER_DIR" -provider oqsprovider -provider default -public-key-algorithms 2>/dev/null | grep -E "(dilithium|falcon)"' >> /usr/local/bin/generate-pqc-cert.sh && \
    echo '  exit 1' >> /usr/local/bin/generate-pqc-cert.sh && \
    echo 'fi' >> /usr/local/bin/generate-pqc-cert.sh && \
    echo '' >> /usr/local/bin/generate-pqc-cert.sh && \
    echo 'echo "Génération de la clé privée..."' >> /usr/local/bin/generate-pqc-cert.sh && \
    echo 'openssl genpkey -provider-path "$PROVIDER_DIR" -provider oqsprovider -provider default -algorithm "$ALGORITHM" -out "${CN}-${ALGORITHM}.key"' >> /usr/local/bin/generate-pqc-cert.sh && \
    echo '' >> /usr/local/bin/generate-pqc-cert.sh && \
    echo 'echo "Génération du certificat..."' >> /usr/local/bin/generate-pqc-cert.sh && \
    echo 'openssl req -provider-path "$PROVIDER_DIR" -provider oqsprovider -provider default -new -x509 -key "${CN}-${ALGORITHM}.key" \' >> /usr/local/bin/generate-pqc-cert.sh && \
    echo '    -out "${CN}-${ALGORITHM}.crt" \' >> /usr/local/bin/generate-pqc-cert.sh && \
    echo '    -days "$DAYS" \' >> /usr/local/bin/generate-pqc-cert.sh && \
    echo '    -subj "/CN=${CN}/O=Test PQC Certificate/C=FR/ST=Paris/L=Paris"' >> /usr/local/bin/generate-pqc-cert.sh && \
    echo '' >> /usr/local/bin/generate-pqc-cert.sh && \
    echo 'echo ""' >> /usr/local/bin/generate-pqc-cert.sh && \
    echo 'echo "✓ Certificat généré avec succès!"' >> /usr/local/bin/generate-pqc-cert.sh && \
    echo 'echo "  Clé privée: ${OUTPUT_DIR}/${CN}-${ALGORITHM}.key"' >> /usr/local/bin/generate-pqc-cert.sh && \
    echo 'echo "  Certificat: ${OUTPUT_DIR}/${CN}-${ALGORITHM}.crt"' >> /usr/local/bin/generate-pqc-cert.sh && \
    echo 'echo ""' >> /usr/local/bin/generate-pqc-cert.sh && \
    echo 'echo "Pour voir les détails du certificat:"' >> /usr/local/bin/generate-pqc-cert.sh && \
    echo 'echo "  openssl x509 -in ${CN}-${ALGORITHM}.crt -text -noout"' >> /usr/local/bin/generate-pqc-cert.sh

RUN chmod +x /usr/local/bin/generate-pqc-cert.sh

# Script pour générer un certificat RSA standard (compatible Go)
RUN echo '#!/bin/bash' > /usr/local/bin/generate-standard-cert.sh && \
    echo '# Script pour générer un certificat RSA standard pour le serveur Go' >> /usr/local/bin/generate-standard-cert.sh && \
    echo 'OUTPUT_DIR=${1:-/certs}' >> /usr/local/bin/generate-standard-cert.sh && \
    echo 'CN=${2:-localhost}' >> /usr/local/bin/generate-standard-cert.sh && \
    echo '' >> /usr/local/bin/generate-standard-cert.sh && \
    echo 'cd "$OUTPUT_DIR"' >> /usr/local/bin/generate-standard-cert.sh && \
    echo 'openssl genrsa -out "${CN}-rsa.key" 2048' >> /usr/local/bin/generate-standard-cert.sh && \
    echo 'openssl req -new -x509 -key "${CN}-rsa.key" \' >> /usr/local/bin/generate-standard-cert.sh && \
    echo '    -out "${CN}-rsa.crt" \' >> /usr/local/bin/generate-standard-cert.sh && \
    echo '    -days 365 \' >> /usr/local/bin/generate-standard-cert.sh && \
    echo '    -subj "/CN=${CN}/O=Test Certificate/C=FR/ST=Paris/L=Paris"' >> /usr/local/bin/generate-standard-cert.sh && \
    chmod +x /usr/local/bin/generate-standard-cert.sh

# Copier et compiler le serveur de test Go
COPY test-server.go /usr/local/bin/test-server.go
RUN cd /usr/local/bin && \
    go build -o test-server test-server.go && \
    rm test-server.go && \
    chmod +x test-server

# Script helper pour démarrer le serveur OpenSSL avec certificat PQC
RUN echo '#!/bin/bash' > /usr/local/bin/start-pqc-server.sh && \
    echo '# Helper script pour démarrer un serveur HTTPS avec certificat PQC (OpenSSL)' >> /usr/local/bin/start-pqc-server.sh && \
    echo 'export OPENSSL_CONF=/etc/ssl/openssl-pqc.cnf' >> /usr/local/bin/start-pqc-server.sh && \
    echo 'CERT_FILE=${1:-/certs/server.crt}' >> /usr/local/bin/start-pqc-server.sh && \
    echo 'KEY_FILE=${2:-/certs/server.key}' >> /usr/local/bin/start-pqc-server.sh && \
    echo 'PORT=${3:-8443}' >> /usr/local/bin/start-pqc-server.sh && \
    echo '' >> /usr/local/bin/start-pqc-server.sh && \
    echo 'if [ ! -f "$CERT_FILE" ] || [ ! -f "$KEY_FILE" ]; then' >> /usr/local/bin/start-pqc-server.sh && \
    echo '  echo "ERREUR: Certificat ou clé introuvable"' >> /usr/local/bin/start-pqc-server.sh && \
    echo '  echo "Usage: start-pqc-server.sh [cert.crt] [key.key] [port]"' >> /usr/local/bin/start-pqc-server.sh && \
    echo '  echo "Par défaut: server.crt server.key 8443"' >> /usr/local/bin/start-pqc-server.sh && \
    echo '  exit 1' >> /usr/local/bin/start-pqc-server.sh && \
    echo 'fi' >> /usr/local/bin/start-pqc-server.sh && \
    echo '' >> /usr/local/bin/start-pqc-server.sh && \
    echo 'echo "🚀 Démarrage du serveur HTTPS avec certificat PQC..."' >> /usr/local/bin/start-pqc-server.sh && \
    echo 'echo "📜 Certificat: $CERT_FILE"' >> /usr/local/bin/start-pqc-server.sh && \
    echo 'echo "🔑 Clé privée: $KEY_FILE"' >> /usr/local/bin/start-pqc-server.sh && \
    echo 'echo "🌐 Port: $PORT"' >> /usr/local/bin/start-pqc-server.sh && \
    echo 'echo "Test: curl -k https://localhost:$PORT/"' >> /usr/local/bin/start-pqc-server.sh && \
    echo 'echo ""' >> /usr/local/bin/start-pqc-server.sh && \
    echo 'echo "Vérification du provider..."' >> /usr/local/bin/start-pqc-server.sh && \
    echo 'openssl list -providers | grep -q oqsprovider && echo "✓ oqs-provider chargé" || echo "⚠ oqs-provider non trouvé"' >> /usr/local/bin/start-pqc-server.sh && \
    echo 'echo ""' >> /usr/local/bin/start-pqc-server.sh && \
    echo 'PROVIDER_PATH=$(find /usr/local -name "oqsprovider.so" 2>/dev/null | head -1)' >> /usr/local/bin/start-pqc-server.sh && \
    echo 'if [ -z "$PROVIDER_PATH" ]; then' >> /usr/local/bin/start-pqc-server.sh && \
    echo '  PROVIDER_PATH="/usr/lib/ssl/ossl-modules/oqsprovider.so"' >> /usr/local/bin/start-pqc-server.sh && \
    echo 'fi' >> /usr/local/bin/start-pqc-server.sh && \
    echo '' >> /usr/local/bin/start-pqc-server.sh && \
    echo 'if [ ! -f "$PROVIDER_PATH" ]; then' >> /usr/local/bin/start-pqc-server.sh && \
    echo '  echo "ERREUR: oqsprovider.so non trouvé"' >> /usr/local/bin/start-pqc-server.sh && \
    echo '  echo "Recherche dans: /usr/local/lib* /usr/lib*"' >> /usr/local/bin/start-pqc-server.sh && \
    echo '  find /usr/local /usr/lib -name "oqsprovider.so" 2>/dev/null | head -5' >> /usr/local/bin/start-pqc-server.sh && \
    echo '  exit 1' >> /usr/local/bin/start-pqc-server.sh && \
    echo 'fi' >> /usr/local/bin/start-pqc-server.sh && \
    echo '' >> /usr/local/bin/start-pqc-server.sh && \
    echo 'PROVIDER_DIR=$(dirname "$PROVIDER_PATH")' >> /usr/local/bin/start-pqc-server.sh && \
    echo 'echo "Provider path: $PROVIDER_PATH"' >> /usr/local/bin/start-pqc-server.sh && \
    echo 'echo "Provider dir: $PROVIDER_DIR"' >> /usr/local/bin/start-pqc-server.sh && \
    echo 'echo ""' >> /usr/local/bin/start-pqc-server.sh && \
    echo 'exec openssl s_server -provider-path "$PROVIDER_DIR" -provider oqsprovider -provider default -accept $PORT -cert "$CERT_FILE" -key "$KEY_FILE" -www' >> /usr/local/bin/start-pqc-server.sh && \
    chmod +x /usr/local/bin/start-pqc-server.sh

# Script helper pour démarrer le serveur Go (certificats classiques uniquement)
RUN echo '#!/bin/bash' > /usr/local/bin/start-test-server.sh && \
    echo '# Helper script pour démarrer un serveur HTTPS de test (Go - certificats RSA/ECDSA uniquement)' >> /usr/local/bin/start-test-server.sh && \
    echo 'CERT_FILE=${1:-/certs/server.crt}' >> /usr/local/bin/start-test-server.sh && \
    echo 'KEY_FILE=${2:-/certs/server.key}' >> /usr/local/bin/start-test-server.sh && \
    echo 'PORT=${3:-8443}' >> /usr/local/bin/start-test-server.sh && \
    echo '' >> /usr/local/bin/start-test-server.sh && \
    echo 'if [ ! -f "$CERT_FILE" ] || [ ! -f "$KEY_FILE" ]; then' >> /usr/local/bin/start-test-server.sh && \
    echo '  echo "ERREUR: Certificat ou clé introuvable"' >> /usr/local/bin/start-test-server.sh && \
    echo '  echo "Usage: start-test-server.sh [cert.crt] [key.key] [port]"' >> /usr/local/bin/start-test-server.sh && \
    echo '  exit 1' >> /usr/local/bin/start-test-server.sh && \
    echo 'fi' >> /usr/local/bin/start-test-server.sh && \
    echo '' >> /usr/local/bin/start-test-server.sh && \
    echo 'echo "Démarrage du serveur HTTPS de test (Go)..."' >> /usr/local/bin/start-test-server.sh && \
    echo 'exec test-server "$CERT_FILE" "$KEY_FILE" "$PORT"' >> /usr/local/bin/start-test-server.sh && \
    chmod +x /usr/local/bin/start-test-server.sh

# Commande par défaut
CMD ["/bin/bash"]


#!/bin/bash
# Script pour builder l'image Docker avec OpenSSL + oqs-provider

set -e

IMAGE_NAME="pqc-openssl"
IMAGE_TAG="latest"
DOCKERFILE="docker/Dockerfile.pqc-openssl"

echo "🔨 Construction de l'image Docker pour OpenSSL + oqs-provider..."
echo ""

docker build -t "${IMAGE_NAME}:${IMAGE_TAG}" -f "${DOCKERFILE}" .

echo ""
echo "✅ Image construite avec succès!"
echo ""
echo "Pour utiliser l'image:"
echo "  docker run -it --rm -v \$(pwd):/certs ${IMAGE_NAME}:${IMAGE_TAG}"
echo ""
echo "Pour générer un certificat:"
echo "  docker run -it --rm -v \$(pwd):/certs ${IMAGE_NAME}:${IMAGE_TAG} \\"
echo "    generate-pqc-cert.sh dilithium3 365 localhost"
echo ""


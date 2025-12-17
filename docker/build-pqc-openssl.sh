#!/bin/bash
# Script to build the Docker image with OpenSSL + oqs-provider

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${PROJECT_ROOT}"

IMAGE_NAME="pqc-openssl"
IMAGE_TAG="latest"
DOCKERFILE="${PROJECT_ROOT}/docker/Dockerfile.pqc-openssl"

echo "Building Docker image for OpenSSL + oqs-provider..."
echo ""

docker build -t "${IMAGE_NAME}:${IMAGE_TAG}" -f "${DOCKERFILE}" .

echo ""
echo "Image successfully built"
echo ""
echo "To use the image:"
echo "  docker run -it --rm -v \$(pwd):/certs ${IMAGE_NAME}:${IMAGE_TAG}"
echo ""
echo "To generate a certificate:"
echo "  docker run -it --rm -v \$(pwd):/certs ${IMAGE_NAME}:${IMAGE_TAG} \\"
echo "    generate-pqc-cert.sh dilithium3 365 localhost"
echo ""


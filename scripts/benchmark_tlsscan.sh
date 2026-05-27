#!/bin/bash

TESTSSL="${HOME}/dev/github/testssl.sh/testssl.sh"

mkdir -p benchmark
cd benchmark

if [ ! -f $TESTSSL ]; then
  echo "[ERROR] File not found: $TESTSSL"
  echo "Please install it from https://github.com/drwetter/testssl.sh"
  echo "And and change the path in the script"
  exit 1
fi

$TESTSSL --help > /dev/null 2>&1
if [ $? -ne 0 ]; then
  echo "[ERROR] Can't run $TESTSSL"
  exit 1
fi

 TOKEN=$(curl -s  -X POST http://localhost:8080/auth/signin \
   -H "Content-Type: application/json" \
   -d '{
    "email": "user@example.com",
    "password": "securepassword",
    "confirm_password": "securepassword",
    "turnstile_token": "0.abcdefghijklmnopqrstuvwxyz..."
  }'| jq -r '.token')


# Canonical TLS list path (v1); legacy GET /discovery/tls/scans was removed (IMM-11).
curl -X GET "http://localhost:8080/discovery/v1/tls/scans?limit=10&offset=0" \
  -H "Authorization: Bearer $TOKEN" | jq . > cafediscovery_tlsscan.json

curl -X GET "http://localhost:8080/discovery/v1/tls/scans?limit=10&offset=10" \
  -H "Authorization: Bearer $TOKEN" | jq . >> cafediscovery_tlsscan.json

jq -r '.items[].endpoint' cafediscovery_tlsscan.json

for url in $(jq -r '.items[].endpoint' cafediscovery_tlsscan.json); do
  echo "Scanning $url"
#  $TESTSSL $url | tee $OUTPUT_FILE
#√ cafe-discovery % ~/dev/github/testssl.sh/testssl.sh  -4 --ip one --json https://mainnet.base.org

  $TESTSSL \
    -4 \
    --ip one \
    --json \
    $url
done

cd - 

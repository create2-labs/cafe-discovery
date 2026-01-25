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


curl -X GET "http://localhost:8080/discovery/tls/scans?limit=10&offset=0" \
  -H "Authorization: Bearer $TOKEN" | jq . > cafediscover_tlsscans.json

jq -r '.results[].url' cafediscover_tlsscans.json

for url in $(jq -r '.results[].url' cafediscover_tlsscans.json); do
  echo "Scanning $url"
  OUTPUT_FILE="${url#https://}.txt"
  echo "Output file: $OUTPUT_FILE"
  $TESTSSL $url | tee $OUTPUT_FILE
done

cd - 
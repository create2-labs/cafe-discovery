#!/usr/bin/env bash
set -e

echo "=== 🧩 Fix OQS Provider linkage and environment on macOS ==="

# Detect paths
OPENSSL_DIR=$(brew --prefix openssl@3)
OQS_LIB_DIR="/opt/liboqs/lib"
OQS_INCLUDE_DIR="/opt/liboqs/include"
OQS_MODULE_DIR="$OPENSSL_DIR/lib/ossl-modules"
OQS_PROVIDER="$OQS_MODULE_DIR/oqsprovider.dylib"

# 1️⃣ Check presence of files
echo "→ Checking files..."
if [ ! -f "$OQS_PROVIDER" ]; then
  echo "❌ Provider not found: $OQS_PROVIDER"
  exit 1
fi
if [ ! -f "$OQS_LIB_DIR/liboqs.dylib" ]; then
  echo "❌ liboqs not found in $OQS_LIB_DIR"
  exit 1
fi
echo "✅ Provider and liboqs found."

# 2️⃣ Add RPATH so the provider can find liboqs
echo "→ Patching oqsprovider.dylib with install_name_tool..."
sudo install_name_tool -add_rpath "$OQS_LIB_DIR" "$OQS_PROVIDER" || true

echo "→ Verifying linkage..."
otool -L "$OQS_PROVIDER" | grep liboqs || echo "⚠️  liboqs not yet visible (will rely on DYLD_LIBRARY_PATH)."

# 3️⃣ Export environment variables
echo "→ Exporting environment variables..."
export DYLD_LIBRARY_PATH="$OQS_LIB_DIR"
export OPENSSL_MODULES="$OQS_MODULE_DIR"
export CGO_CFLAGS="-I$OPENSSL_DIR/include -I$OQS_INCLUDE_DIR"
export CGO_LDFLAGS="-L$OPENSSL_DIR/lib -L$OQS_LIB_DIR -lssl -lcrypto -loqs"

cat <<EOF

✅ Environment configured:
  DYLD_LIBRARY_PATH=$DYLD_LIBRARY_PATH
  OPENSSL_MODULES=$OPENSSL_MODULES
  CGO_CFLAGS=$CGO_CFLAGS
  CGO_LDFLAGS=$CGO_LDFLAGS

EOF

# 4️⃣ Test OpenSSL provider loading
echo "→ Testing provider loading..."
$OPENSSL_DIR/bin/openssl list -providers -provider oqsprovider || {
  echo "⚠️  Could not load oqsprovider — will retry with -provider-path"
  $OPENSSL_DIR/bin/openssl list -providers \
    -provider-path "$OQS_MODULE_DIR" \
    -provider oqsprovider
}

# 5️⃣ List groups available via OQS provider
echo "→ Listing available PQC/hybrid groups:"
$OPENSSL_DIR/bin/openssl list -groups -provider oqsprovider || echo "⚠️  Unable to list groups — check provider load above."

echo
echo "✅ Done. You can now run:"
echo "   go build -v -o pq-scan main.go c_kex.c"
echo "   ./pq-scan https://pq.cloudflareresearch.com"

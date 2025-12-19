#!/usr/bin/env bash
set -e

echo "🔧 Installing liboqs + oqs-provider (hybrid-enabled) on macOS (ARM64)..."

# --- CONFIG ---
LIBOQS_PREFIX=/opt/liboqs
OPENSSL_PREFIX=/opt/homebrew/etc/openssl@3
BUILD_DIR=$HOME/tmp/oqs_build

# --- PREREQUISITES ---
echo "📦 Installing prerequisites..."
brew install cmake ninja git

mkdir -p "$BUILD_DIR"
cd "$BUILD_DIR"

# --- STEP 1: Build liboqs ---
if [ ! -d "$LIBOQS_PREFIX/lib" ]; then
  echo "📦 Building liboqs with hybrid KEM support..."
  git clone --depth 1 https://github.com/open-quantum-safe/liboqs.git
  cd liboqs
  mkdir -p build && cd build
  cmake -GNinja \
    -DBUILD_SHARED_LIBS=ON \
    -DOQS_ENABLE_KEM_HYBRID=ON \
    -DCMAKE_INSTALL_PREFIX="$LIBOQS_PREFIX" \
    -DCMAKE_BUILD_TYPE=Release ..
  ninja
  sudo ninja install
  cd "$BUILD_DIR"
else
  echo "✅ liboqs already installed in $LIBOQS_PREFIX"
fi

# --- STEP 2: Build oqs-provider ---
echo "📦 Building oqs-provider..."
git clone --depth 1 https://github.com/open-quantum-safe/oqs-provider.git
cd oqs-provider
mkdir -p build && cd build

cmake -GNinja \
  -DOQS_INSTALL_PATH="$LIBOQS_PREFIX" \
  -DOPENSSL_ROOT_DIR="$OPENSSL_PREFIX" \
  -DCMAKE_BUILD_TYPE=Release ..
ninja
sudo ninja install

# --- STEP 3: Environment setup ---
echo "🧩 Configuring environment variables..."

cat <<EOF | sudo tee /etc/profile.d/oqs-provider.sh >/dev/null
# OQS provider environment
export OPENSSL_MODULES=$OPENSSL_PREFIX/lib/ossl-modules
export OPENSSL_CONF=$OPENSSL_PREFIX/ssl/openssl.cnf
export DYLD_LIBRARY_PATH=$LIBOQS_PREFIX/lib:\$DYLD_LIBRARY_PATH
EOF

# Apply to current shell session
export OPENSSL_MODULES=$OPENSSL_PREFIX/lib/ossl-modules
export OPENSSL_CONF=$OPENSSL_PREFIX/ssl/openssl.cnf
export DYLD_LIBRARY_PATH=$LIBOQS_PREFIX/lib:$DYLD_LIBRARY_PATH

echo "✅ Installation completed. Verifying..."
"$OPENSSL_PREFIX/bin/openssl" list -providers | grep OQS && echo "✅ OQS provider active" || echo "⚠️ OQS provider not detected"
"$OPENSSL_PREFIX/bin/openssl" list -key-exchange-algorithms -provider oqsprovider -provider default | grep MLKEM || echo "⚠️ Hybrid groups not detected"

echo
echo "🎉 Done! You can now test with:"
echo "    $OPENSSL_PREFIX/bin/openssl s_client -provider oqsprovider -provider default -groups SecP256r1MLKEM768 -connect test.openquantumsafe.org:6001"


# Install liboqs-go
git clone --depth=1 https://github.com/open-quantum-safe/liboqs-go
cd liboqs-go
cat  .config/liboqs-go.pc| sed "s/LIBOQS_INCLUDE_DIR=.*/LIBOQS_INCLUDE_DIR=\/usr\/local\/oqs-openssl\/include"     > temp.pc
mv temp.pc .config/liboqs-go.pc
cat  .config/liboqs-go.pc| sed "s/LIBOQS_LIB_DIR=.*/LIBOQS_LIB_DIR=\/usr\/local\/oqs-openssl\/usr\/local\/lib"     > temp.pc
mv temp.pc .config/liboqs-go.pc


echo "export PKG_CONFIG_PATH=$PKG_CONFIG_PATH:$(pwd)/liboqs-go/.config" >>  /etc/profile.d/oqs-provider.sh
export PKG_CONFIG_PATH=$PKG_CONFIG_PATH:$(pwd)/liboqs-go/.config
go run examples/kem/kem.go && echo "✅ OQS Golang bindings installed" || echo "⚠️ OQS Golang bindings installation error"

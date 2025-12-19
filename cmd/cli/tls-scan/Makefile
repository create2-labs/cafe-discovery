
# Makefile for pq-scan (Go + CGO + OpenSSL/OQS-OpenSSL)
BINARY := pq-scan

# If pkg-config for openssl is not available or points to a wrong OpenSSL, set these:
# export CGO_CFLAGS="-I/opt/oqs-openssl/include"
# export CGO_LDFLAGS="-L/opt/oqs-openssl/lib -lssl -lcrypto -loqs"

.PHONY: all build run clean

all: build

build:
	@echo "Building $(BINARY) (CGO with OpenSSL). Ensure OpenSSL (and OQS-OpenSSL if needed) are installed."
	CGO_ENABLED=1 go build -v -o $(BINARY) main.go c_kex.c

run: build
	@if [ -z "$(TARGET)" ]; then \
		echo "Usage: make run TARGET=host[:port]"; exit 1; \
	fi
	./$(BINARY) $(TARGET)

clean:
	-rm -f $(BINARY)

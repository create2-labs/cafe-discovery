# Makefile to build tls_pqc_scan  (CAFE native tests)
# Compatible MacOS & Linux
# Requirements:
#   - OpenSSL@3 (macOS: brew install openssl@3, Linux: apt install libssl-dev)
#   - libpcap (macOS: brew install libpcap, Linux: apt install libpcap-dev)

# Detect OS
UNAME_S := $(shell uname -s)

# macOS detection
ifeq ($(UNAME_S),Darwin)
    # macOS: use Homebrew paths
    # Apple Silicon (arm64) or Intel (x86_64)
    HOMEBREW_PREFIX ?= /opt/homebrew
    OPENSSL_PREFIX ?= $(HOMEBREW_PREFIX)/opt/openssl@3
    PCAP_LDFLAGS := -L$(HOMEBREW_PREFIX)/lib -lpcap
else
    # Linux: use system paths
    OPENSSL_PREFIX ?= /usr
    PCAP_LDFLAGS := -lpcap
endif

# Compiler flags
CFLAGS  += -I$(OPENSSL_PREFIX)/include -O2 -Wall -Wextra -std=c11
CPPFLAGS  += -I$(OPENSSL_PREFIX)/include -O2 -Wall -Wextra -std=c++11
LDFLAGS += -L$(OPENSSL_PREFIX)/lib -lssl -lcrypto

all: tls_pqc_scan

clean:
	rm -f  tls_pqc_scan

tls_pqc_scan: tls_pqc_scan.c
	$(CC) -DTEST_MAIN $^ -o $@ $(CFLAGS) $(LDFLAGS)


tests:
	./tls_pqc_scan test.openquantumsafe.org || true
	./tls_pqc_scan test.openquantumsafe.org:6001 --group SecP256r1MLKEM768 || true
	./tls_pqc_scan test.openquantumsafe.org:6002 --group SecP384r1MLKEM1024 || true
	./tls_pqc_scan test.openquantumsafe.org:6003 --group X25519MLKEM768 || true

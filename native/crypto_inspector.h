// crypto_inspector.h
#pragma once

#include <string>
#include <vector>
#include <cstdint>

struct HybridGroupInfo {
    uint16_t group_id;          // TLS NamedGroup ID
    std::string group_name;     // e.g. "X25519MLKEM768"
    std::string oqs_kem_name;   // e.g. "ML-KEM-768"
    int nist_level;             // e.g. 1, 3, 5
};

struct CryptoAssessment {
    bool is_tls = false;
    std::string sni; // Server Name Indication
    bool is_client_hello = false;
    std::string tls_version;               // "TLS1.2", "TLS1.3", ...
    std::vector<std::string> cipher_suites;
    std::vector<HybridGroupInfo> hybrids;  // hybrid KEM groups detected
    bool has_pqc = false;
    bool has_hybrid = false;
    int max_nist_level = 0;                // 0 = pre-quantum only
};

// Analyze a TCP payload supposed to be a TLS record.
// Fill assessment if it's a ClientHello, return true if TLS detected.
bool analyze_tls_client_hello(const uint8_t* data, int len, CryptoAssessment& assessment);

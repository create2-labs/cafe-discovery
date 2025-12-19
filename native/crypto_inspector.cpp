// crypto_inspector.cpp
#include "crypto_inspector.h"

#include <cstring>
#include <unordered_map>
#include <cstdio>

#ifdef USE_LIBOQS
#include <oqs/oqs.h>
#endif

// ===== TLS constants =====
static const uint8_t TLS_CONTENT_TYPE_HANDSHAKE = 0x16;
static const uint8_t TLS_HANDSHAKE_CLIENT_HELLO = 0x01;

// Extensions
static const uint16_t TLS_EXT_SUPPORTED_GROUPS = 0x000a;

// Quelques cipher suites en texte (optionnel)
static std::string cipher_suite_to_string(uint16_t id) {
    switch (id) {
        case 0x1301: return "TLS_AES_128_GCM_SHA256";
        case 0x1302: return "TLS_AES_256_GCM_SHA384";
        case 0x1303: return "TLS_CHACHA20_POLY1305_SHA256";
        default: {
            char buf[16];
            std::snprintf(buf, sizeof(buf), "0x%04x", id);
            return std::string(buf);
        }
    }
}

// ---- Table des groupes hybrides connus ----
// IDs à ADAPTER à ta stack (oqs-provider, etc.).
static const std::unordered_map<uint16_t, HybridGroupInfo> HYBRID_GROUPS = {
    // { NamedGroup ID, { id,         label,             OQS KEM,      NIST lvl } }
    { 0xfe30, { 0xfe30, "X25519MLKEM768",   "ML-KEM-768",   3 } },
    { 0xfe31, { 0xfe31, "P256MLKEM512",     "ML-KEM-512",   1 } },
    { 0xfe32, { 0xfe32, "X25519MLKEM1024",  "ML-KEM-1024",  5 } }
};

static int get_nist_level_from_oqs(const std::string& kem_name, int fallback) {
#ifdef USE_LIBOQS
    OQS_KEM* kem = OQS_KEM_new(kem_name.c_str());
    if (!kem) return fallback;
    int level = kem->claimed_nist_level;
    OQS_KEM_free(kem);
    return level;
#else
    (void)kem_name;
    return fallback;
#endif
}

static std::string tls_version_to_string(uint16_t version) {
    switch (version) {
        case 0x0301: return "TLS1.0";
        case 0x0302: return "TLS1.1";
        case 0x0303: return "TLS1.2";
        case 0x0304: return "TLS1.3";
        default: {
            char buf[16];
            std::snprintf(buf, sizeof(buf), "0x%04x", version);
            return std::string(buf);
        }
    }
}

static uint16_t read_u16(const uint8_t* p) {
    return (static_cast<uint16_t>(p[0]) << 8) | p[1];
}

// Parse l’extension "supported_groups"
static void parse_supported_groups(const uint8_t* ext_data, uint16_t ext_len,
                                   CryptoAssessment& assessment) {
    if (ext_len < 2) return;

    uint16_t list_len = read_u16(ext_data);
    if (2 + list_len > ext_len) return;

    const uint8_t* p   = ext_data + 2;
    const uint8_t* end = ext_data + 2 + list_len;

    while (p + 2 <= end) {
        uint16_t group_id = read_u16(p);
        p += 2;

        auto it = HYBRID_GROUPS.find(group_id);
        if (it != HYBRID_GROUPS.end()) {
            HybridGroupInfo info = it->second;
            info.nist_level = get_nist_level_from_oqs(info.oqs_kem_name, info.nist_level);

            assessment.hybrids.push_back(info);
            assessment.has_pqc    = true;
            assessment.has_hybrid = true;
            if (info.nist_level > assessment.max_nist_level)
                assessment.max_nist_level = info.nist_level;
        }
    }
}

// Parse le bloc Extensions du ClientHello
static void parse_extensions(const uint8_t* p, const uint8_t* end,
                             CryptoAssessment& assessment) {
    if (p + 2 > end) return;

    static const uint16_t TLS_EXT_SERVER_NAME = 0x0000;

    uint16_t ext_total_len = read_u16(p);
    p += 2;
    const uint8_t* ext_end = p + ext_total_len;
    if (ext_end > end) return;

    while (p + 4 <= ext_end) {
        uint16_t ext_type = read_u16(p);
        uint16_t ext_len  = read_u16(p + 2);
        p += 4;

        if (p + ext_len > ext_end) break;

        const uint8_t* ext_data = p;

        if (ext_type == TLS_EXT_SERVER_NAME) {
            // Format:
            //   ServerNameList {
            //     uint16 list_len
            //     ServerName {
            //        uint8  name_type (0=host_name)
            //        uint16 name_len
            //        opaque name[name_len]
            //     }
            //   }
            if (ext_len >= 5) { // minimal size
                uint16_t list_len = read_u16(ext_data);
                const uint8_t* p2 = ext_data + 2;
                const uint8_t* end2 = ext_data + ext_len;
        
                if (p2 + list_len <= end2 && list_len >= 3) {
                    uint8_t name_type = p2[0];
                    uint16_t name_len = read_u16(p2 + 1);
                    const uint8_t* name_ptr = p2 + 3;
        
                    if (name_type == 0 && name_ptr + name_len <= end2) {
                        assessment.sni = std::string((const char*)name_ptr, name_len);
                    }
                }
            }
        }

        if (ext_type == TLS_EXT_SUPPORTED_GROUPS) {
            parse_supported_groups(ext_data, ext_len, assessment);
        }
        
        p += ext_len;
    }
}

// Parse le ClientHello (sans l’entête Record TLS)
static void parse_client_hello(const uint8_t* p, int len,
                               CryptoAssessment& assessment) {
    const uint8_t* end = p + len;
    if (p + 4 > end) return;

    uint8_t  msg_type = p[0];
    uint32_t msg_len  = (static_cast<uint32_t>(p[1]) << 16) |
                        (static_cast<uint32_t>(p[2]) << 8)  |
                        (static_cast<uint32_t>(p[3]));
    p += 4;

    if (msg_type != TLS_HANDSHAKE_CLIENT_HELLO) return;
    if (p + msg_len > end) return;

    assessment.is_client_hello = true;

    if (p + 2 > end) return;
    uint16_t client_version = read_u16(p);
    p += 2;
    assessment.tls_version = tls_version_to_string(client_version);

    // Random (32)
    if (p + 32 > end) return;
    p += 32;

    // Session ID
    if (p + 1 > end) return;
    uint8_t sid_len = p[0];
    p += 1;
    if (p + sid_len > end) return;
    p += sid_len;

    // Cipher Suites
    if (p + 2 > end) return;
    uint16_t cs_len = read_u16(p);
    p += 2;
    if (p + cs_len > end) return;

    const uint8_t* cs_end = p + cs_len;
    while (p + 2 <= cs_end) {
        uint16_t cs_id = read_u16(p);
        p += 2;
        assessment.cipher_suites.push_back(cipher_suite_to_string(cs_id));
    }

    // Compression
    if (p + 1 > end) return;
    uint8_t comp_len = p[0];
    p += 1;
    if (p + comp_len > end) return;
    p += comp_len;

    // Extensions
    if (p + 2 > end) return;
    parse_extensions(p, end, assessment);
}

bool analyze_tls_client_hello(const uint8_t* data, int len,
                              CryptoAssessment& assessment) {
    if (len < 5) return false;
    if (data[0] != TLS_CONTENT_TYPE_HANDSHAKE) return false;

    assessment.is_tls = true;

    uint16_t version = read_u16(data + 1);
    (void)version; // éventuellement utile plus tard
    uint16_t rec_len = read_u16(data + 3);

    if (5 + rec_len > len) {
        // Paquet tronqué, mais on sait que c’est du TLS
        return true;
    }

    const uint8_t* p   = data + 5;
    int            hs_len = rec_len;

    parse_client_hello(p, hs_len, assessment);
    return true;
}

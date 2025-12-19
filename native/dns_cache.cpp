// dns_cache.cpp
#include "dns_cache.h"
#include <netdb.h>
#include <arpa/inet.h>

DNSCache::DNSCache(size_t maxSize, int ttlSeconds)
    : maxSize_(maxSize), ttl_(ttlSeconds) {}

std::string DNSCache::lookup(const std::string& ip) {
    struct sockaddr_in sa {};
    sa.sin_family = AF_INET;

    if (inet_pton(AF_INET, ip.c_str(), &sa.sin_addr) <= 0)
        return "";

    char host[NI_MAXHOST];
    int r = getnameinfo(
        (struct sockaddr*)&sa,
        sizeof(sa),
        host,
        sizeof(host),
        nullptr,
        0,
        NI_NAMEREQD
    );

    return (r == 0) ? std::string(host) : "";
}

void DNSCache::evictIfNeeded() {
    if (cache_.size() <= maxSize_) return;

    // stratégie simple : effacer la première entrée (LRU plus tard si tu veux)
    auto it = cache_.begin();
    if (it != cache_.end())
        cache_.erase(it);
}

std::string DNSCache::resolve(const std::string& ip) {
    std::lock_guard<std::mutex> lock(mutex_);

    auto now = std::chrono::steady_clock::now();

    // 1. check cache
    auto it = cache_.find(ip);
    if (it != cache_.end()) {
        if (it->second.expiry > now) {
            return it->second.hostname;
        }
        // expired → remove
        cache_.erase(it);
    }

    // 2. real DNS lookup (slow)
    std::string h = lookup(ip);

    // 3. store in cache
    Entry e;
    e.hostname = h;
    e.expiry = now + std::chrono::seconds(ttl_);

    cache_[ip] = e;

    // 4. evict if needed
    evictIfNeeded();

    return h;
}

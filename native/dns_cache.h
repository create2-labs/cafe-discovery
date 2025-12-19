// dns_cache.h
#pragma once
#include <string>
#include <unordered_map>
#include <chrono>
#include <mutex>

class DNSCache {
public:
    DNSCache(size_t maxSize = 256, int ttlSeconds = 300);
    std::string resolve(const std::string& ip);

private:
    struct Entry {
        std::string hostname;
        std::chrono::steady_clock::time_point expiry;
    };

    size_t maxSize_;
    int ttl_;
    std::unordered_map<std::string, Entry> cache_;
    std::mutex mutex_;

    std::string lookup(const std::string& ip); // real DNS lookup
    void evictIfNeeded();
};

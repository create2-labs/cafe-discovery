// Simple HTTPS server using OpenSSL with PQC certificate support
// Compile with: gcc -o pqc-server pqc-server.c -lssl -lcrypto -ldl -lpthread

#define _GNU_SOURCE
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <sys/socket.h>
#include <netinet/in.h>
#include <arpa/inet.h>
#include <openssl/ssl.h>
#include <openssl/err.h>
#include <openssl/provider.h>

int create_socket(int port) {
    int s;
    struct sockaddr_in addr;

    addr.sin_family = AF_INET;
    addr.sin_port = htons(port);
    addr.sin_addr.s_addr = htonl(INADDR_ANY);

    s = socket(AF_INET, SOCK_STREAM, 0);
    if (s < 0) {
        perror("Unable to create socket");
        exit(EXIT_FAILURE);
    }

    if (bind(s, (struct sockaddr*)&addr, sizeof(addr)) < 0) {
        perror("Unable to bind");
        exit(EXIT_FAILURE);
    }

    if (listen(s, 1) < 0) {
        perror("Unable to listen");
        exit(EXIT_FAILURE);
    }

    return s;
}

SSL_CTX *create_context() {
    const SSL_METHOD *method;
    SSL_CTX *ctx;

    method = TLS_server_method();
    ctx = SSL_CTX_new(method);
    if (!ctx) {
        perror("Unable to create SSL context");
        ERR_print_errors_fp(stderr);
        exit(EXIT_FAILURE);
    }

    return ctx;
}

void configure_context(SSL_CTX *ctx, const char *cert_file, const char *key_file) {
    // Load certificate
    if (SSL_CTX_use_certificate_file(ctx, cert_file, SSL_FILETYPE_PEM) <= 0) {
        ERR_print_errors_fp(stderr);
        exit(EXIT_FAILURE);
    }

    // Load private key
    if (SSL_CTX_use_PrivateKey_file(ctx, key_file, SSL_FILETYPE_PEM) <= 0) {
        ERR_print_errors_fp(stderr);
        exit(EXIT_FAILURE);
    }
}

void handle_client(SSL *ssl) {
    const char response[] = "HTTP/1.1 200 OK\r\n"
                           "Content-Type: text/plain\r\n"
                           "Content-Length: 3\r\n"
                           "Connection: close\r\n\r\n"
                           "ok\n";

    SSL_write(ssl, response, strlen(response));
}

int main(int argc, char **argv) {
    const char *cert_file = "/certs/server.crt";
    const char *key_file = "/certs/server.key";
    int port = 8443;

    if (argc > 1) cert_file = argv[1];
    if (argc > 2) key_file = argv[2];
    if (argc > 3) port = atoi(argv[3]);

    printf("🚀 Démarrage serveur HTTPS PQC sur le port %d\n", port);
    printf("📜 Certificat: %s\n", cert_file);
    printf("🔑 Clé: %s\n", key_file);
    printf("\n");

    // Load providers
    OSSL_PROVIDER *prov_default = OSSL_PROVIDER_load(NULL, "default");
    OSSL_PROVIDER *prov_oqs = OSSL_PROVIDER_load(NULL, "oqsprovider");

    if (!prov_default || !prov_oqs) {
        fprintf(stderr, "Erreur: Impossible de charger les providers\n");
        ERR_print_errors_fp(stderr);
        exit(EXIT_FAILURE);
    }

    SSL_load_error_strings();
    OpenSSL_add_ssl_algorithms();

    SSL_CTX *ctx = create_context();
    configure_context(ctx, cert_file, key_file);

    int sock = create_socket(port);

    printf("✓ Serveur démarré. Test: curl -k https://localhost:%d/\n\n", port);

    struct sockaddr_in addr;
    unsigned int len = sizeof(addr);

    while (1) {
        int client = accept(sock, (struct sockaddr*)&addr, &len);
        if (client < 0) {
            perror("Unable to accept");
            exit(EXIT_FAILURE);
        }

        SSL *ssl = SSL_new(ctx);
        SSL_set_fd(ssl, client);

        if (SSL_accept(ssl) <= 0) {
            ERR_print_errors_fp(stderr);
        } else {
            handle_client(ssl);
        }

        SSL_shutdown(ssl);
        SSL_free(ssl);
        close(client);
    }

    close(sock);
    SSL_CTX_free(ctx);
    OSSL_PROVIDER_unload(prov_default);
    OSSL_PROVIDER_unload(prov_oqs);
    EVP_cleanup();

    return 0;
}




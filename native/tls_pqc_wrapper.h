#ifndef TLS_PQC_WRAPPER_H
#define TLS_PQC_WRAPPER_H

#include <stdbool.h>

// Forward declaration of get_pqc_info from tls_pqc_scan.c
char *get_pqc_info(const char *host, const char *port, const char *grp, bool trace);

#endif // TLS_PQC_WRAPPER_H


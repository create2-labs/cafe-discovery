// Wrapper to include tls_pqc_scan.c only once
// This prevents duplicate symbols when multiple packages use it
// TEST_MAIN is NOT defined, so main() won't be compiled

// Guard to prevent multiple inclusions - this file is included via #include in native.go
#ifndef TLS_PQC_WRAPPER_IMPL
#define TLS_PQC_WRAPPER_IMPL

// Ensure TEST_MAIN is not defined to prevent main() from being compiled
#ifdef TEST_MAIN
#undef TEST_MAIN
#endif

// Include the implementation
#include "tls_pqc/tls_pqc_scan.c"

#endif // TLS_PQC_WRAPPER_IMPL


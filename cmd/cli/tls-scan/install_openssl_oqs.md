# Preparer une distro OpenSSL + OQS

## Builder les packages

- Pour Debian , utiliser le script `install_oqs_openssl_debian.sh`
- Pour MacOS , utiliser le script `install_oqs_openssl_mac.sh`

## Build

- Builder l'image de build avec `Dockerfile-oqs.builder`
```
docker build [--no-cache] -t oqs-builder -f Dockerfile-oqs.builder .
```

- Builder l'image runtime avec  `Dockerfile-oqs.runtime`
```
docker build [--no-cache] -t oqs-runtime -f Dockerfile-oqs.runtime .
```

## Run
```
docker run -ti oqs-runtime bash
```
#!/bin/sh
set -e

apk add --no-cache openssl > /dev/null 2>&1

CERT_DIR=/etc/nginx/certs
CERT_KEY=$CERT_DIR/server.key
CERT_PEM=$CERT_DIR/server.crt

if [ ! -f "$CERT_KEY" ] || [ ! -f "$CERT_PEM" ]; then
    mkdir -p "$CERT_DIR"
    openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
        -keyout "$CERT_KEY" \
        -out "$CERT_PEM" \
        -subj "/CN=ft-hackthon.local/O=ft_hackthon/C=FR" \
        -addext "subjectAltName=DNS:localhost,DNS:api,IP:127.0.0.1"
    echo "Generated self-signed certificate"
fi

exec nginx -g "daemon off;"

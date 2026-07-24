#!/bin/sh
set -e

CERT_DIR=/etc/nginx/certs
CERT_KEY=$CERT_DIR/server.key
CERT_PEM=$CERT_DIR/server.crt
ACME_HOME=/etc/nginx/acme
ACME_WEBROOT=/var/www/acme

if [ -n "$DOMAIN" ]; then
    echo "Domain set: $DOMAIN — obtaining Let's Encrypt certificate..."

    apk add --no-cache openssl curl > /dev/null 2>&1

    mkdir -p "$ACME_HOME" "$ACME_WEBROOT"
    curl -s https://get.acme.sh | sh -s email=admin@"$DOMAIN" > /dev/null 2>&1
    export LE_WORKING_DIR="$ACME_HOME"

    # Start nginx briefly to serve the ACME challenge
    nginx -g "daemon off;" &
    NGINX_PID=$!
    sleep 1

    # Issue cert via webroot
    "$ACME_HOME/acme.sh" --issue --webroot "$ACME_WEBROOT" -d "$DOMAIN" --force

    # Install cert
    "$ACME_HOME/acme.sh" --install-cert -d "$DOMAIN" \
        --key-file "$CERT_KEY" \
        --fullchain-file "$CERT_PEM" \
        --reloadcmd "nginx -s reload"

    echo "Certificate obtained for $DOMAIN — renewals handled by acme.sh cron"

    # Kill the temporary nginx, main process will start below
    kill "$NGINX_PID" 2>/dev/null
    wait "$NGINX_PID" 2>/dev/null

elif [ ! -f "$CERT_KEY" ] || [ ! -f "$CERT_PEM" ]; then
    apk add --no-cache openssl > /dev/null 2>&1
    mkdir -p "$CERT_DIR"
    openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
        -keyout "$CERT_KEY" \
        -out "$CERT_PEM" \
        -subj "/CN=ft-hackthon.local/O=ft_hackthon/C=FR" \
        -addext "subjectAltName=DNS:localhost,DNS:api,IP:127.0.0.1"
    echo "Generated self-signed certificate"
fi

exec nginx -g "daemon off;"

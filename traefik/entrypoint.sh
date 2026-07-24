#!/bin/sh
set -e

apk add --no-cache openssl > /dev/null 2>&1

CERT_DIR=/etc/traefik/certs
CERT_KEY=$CERT_DIR/server.key
CERT_PEM=$CERT_DIR/server.crt

# Static traefik.yml
cat > /etc/traefik/traefik.yml <<EOF
entryPoints:
  web:
    address: ":80"
    http:
      redirections:
        entryPoint:
          to: websecure
          scheme: https
  websecure:
    address: ":8443"

providers:
  file:
    filename: /etc/traefik/dynamic.yml
EOF

if [ -n "$DOMAIN" ]; then
  # ACME mode
  cat >> /etc/traefik/traefik.yml <<EOF

certificatesResolvers:
  le:
    acme:
      email: admin@$DOMAIN
      storage: /etc/traefik/acme.json
      httpChallenge:
        entryPoint: web
EOF

  cat > /etc/traefik/dynamic.yml <<EOF
http:
  routers:
    api:
      rule: "Host(\`$DOMAIN\`)"
      service: api
      entryPoints:
        - websecure
      tls:
        certResolver: le

  services:
    api:
      loadBalancer:
        servers:
          - url: "http://api:8000"
EOF
  echo "Let's Encrypt enabled for $DOMAIN"
else
  # Self-signed mode
  mkdir -p "$CERT_DIR"
  if [ ! -f "$CERT_KEY" ] || [ ! -f "$CERT_PEM" ]; then
    openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
      -keyout "$CERT_KEY" \
      -out "$CERT_PEM" \
      -subj "/CN=ft-hackthon.local/O=ft_hackthon/C=FR" \
      -addext "subjectAltName=DNS:localhost"
    echo "Generated self-signed certificate"
  fi

  cat > /etc/traefik/dynamic.yml <<EOF
tls:
  certificates:
    - certFile: $CERT_PEM
      keyFile: $CERT_KEY

http:
  routers:
    api:
      rule: "HostRegexp('.+')"
      service: api
      entryPoints:
        - websecure
      tls: {}

  services:
    api:
      loadBalancer:
        servers:
          - url: "http://api:8000"
EOF
fi

exec traefik
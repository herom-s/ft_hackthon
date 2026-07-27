#!/bin/sh
set -e

NAMECOM_USERNAME="$1"
NAMECOM_TOKEN="$2"
DOMAIN="$3"
IP="$4"

if [ -z "$NAMECOM_USERNAME" ] || [ -z "$NAMECOM_TOKEN" ] || [ -z "$DOMAIN" ] || [ -z "$IP" ]; then
  echo "Usage: $0 <username> <token> <domain> <ip>"
  echo "Error: all arguments are required (ip was empty)"
  exit 1
fi

# Validate IP looks like an address
case "$IP" in
  *[!0-9.]*) echo "Error: invalid IP '$IP'" >&2; exit 1 ;;
esac

API="https://api.name.com/v4"
AUTH="$NAMECOM_USERNAME:$NAMECOM_TOKEN"

list_records() {
  curl -s -u "$AUTH" "$API/domains/$DOMAIN/records?perPage=1000"
}

find_record() {
  list_records | python3 -c "
import sys, json
data = json.load(sys.stdin)
host = '$1'
for r in data.get('records', []):
    if r.get('host') == host and r.get('type') == 'A':
        print(r['id'], r.get('answer',''))
"
}

upsert_record() {
  local host="$1"
  local existing
  existing=$(find_record "$host")
  local id
  id=$(echo "$existing" | awk '{print $1}')
  local old_ip
  old_ip=$(echo "$existing" | awk '{print $2}')

  if [ -n "$id" ] && [ "$old_ip" = "$IP" ]; then
    echo "  $host.$DOMAIN -> $IP (up-to-date)"
    return 0
  fi

  local host_field
  if [ "$host" = "@" ]; then
    host_field=""
  else
    host_field="$host"
  fi

  local payload
  payload=$(printf '{"host":"%s","type":"A","answer":"%s","ttl":300}' "$host_field" "$IP")

  if [ -n "$id" ]; then
    echo "  Updating $host.$DOMAIN -> $IP (was $old_ip)"
    curl -s -u "$AUTH" -X PUT "$API/domains/$DOMAIN/records/$id" \
      -H "Content-Type: application/json" -d "$payload" > /dev/null
  else
    echo "  Creating $host.$DOMAIN -> $IP"
    curl -s -u "$AUTH" -X POST "$API/domains/$DOMAIN/records" \
      -H "Content-Type: application/json" -d "$payload" > /dev/null
  fi
}

echo "DNS: $DOMAIN -> $IP"
upsert_record "@"
upsert_record "www"
echo "Done."

#!/bin/sh
# links.sh <file.md>: checks every http(s) URL in the body (before "## Verification"); prints only failures.
f="$1"
sed '/^## Verification/,$d' "$f" | grep -oE 'https?://[^ )>|`"]+' | sed 's/[.,;:]*$//' | sort -u | while read -r u; do
  code=$(curl -s -o /dev/null -L --max-time 15 -A "Mozilla/5.0 (loops link check)" -w '%{http_code}' "$u" 2>/dev/null)
  case "$code" in 2??|3??) ;; 401|403|429|999) echo "blocked $code $u";; *) echo "FAIL $code $u";; esac
done

#!/usr/bin/env bash
set -euo pipefail

interval="${1:-5}"
iface_filter="${2:-}"

if ! [[ "$interval" =~ ^[0-9]+$ ]] || [[ "$interval" -le 0 ]]; then
  echo "usage: $0 [interval_seconds] [iface_regex]" >&2
  exit 1
fi

read_counters() {
  awk '
    NR > 2 {
      gsub(":", "", $1);
      print $1, $2, $10;
    }
  ' /proc/net/dev
}

declare -A prev_rx prev_tx

while true; do
  stamp="$(date '+%F %T')"
  while read -r iface rx tx; do
    if [[ -n "$iface_filter" ]] && ! [[ "$iface" =~ $iface_filter ]]; then
      continue
    fi

    prev_rx_value="${prev_rx[$iface]:-}"
    prev_tx_value="${prev_tx[$iface]:-}"
    if [[ -n "$prev_rx_value" && -n "$prev_tx_value" ]]; then
      rx_delta=$((rx - prev_rx_value))
      tx_delta=$((tx - prev_tx_value))
      printf '%s %-12s rx/s=%12s tx/s=%12s total_rx=%12s total_tx=%12s\n' \
        "$stamp" "$iface" "$rx_delta" "$tx_delta" "$rx" "$tx"
    else
      printf '%s %-12s rx/s=%12s tx/s=%12s total_rx=%12s total_tx=%12s\n' \
        "$stamp" "$iface" 0 0 "$rx" "$tx"
    fi

    prev_rx[$iface]="$rx"
    prev_tx[$iface]="$tx"
  done < <(read_counters | sort -k3,3nr)

  sleep "$interval"
done

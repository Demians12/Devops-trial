#!/usr/bin/env bash
set -euo pipefail

log() {
  printf '    - %s\n' "$1"
}

add_repo() {
  local name="$1"
  local url="$2"
  helm repo add "$name" "$url" --force-update >/dev/null
  log "Repositório '$name' sincronizado"
}

log "Sincronizando repositórios Helm necessários"
add_repo prometheus-community https://prometheus-community.github.io/helm-charts
add_repo grafana https://grafana.github.io/helm-charts
add_repo ingress-nginx https://kubernetes.github.io/ingress-nginx
helm repo update >/dev/null
log "Índices atualizados"
#!/usr/bin/env bash
set -euo pipefail

log_step() {
  printf '      * %s\n' "$1"
}

fail() {
  printf '[preflight] %s\n' "$1" >&2
  exit 1
}

REQUIRED_BINS=(docker kind kubectl helm)
log_step "Checando binários obrigatórios"
for bin in "${REQUIRED_BINS[@]}"; do
  if ! command -v "$bin" >/dev/null 2>&1; then
    fail "comando obrigatório ausente: $bin"
  fi
done

log_step "Validando acesso ao daemon Docker"
if ! docker info >/dev/null 2>&1; then
  fail "não foi possível falar com o daemon do Docker. Verifique se está ativo e com permissões corretas."
fi

log_step "Verificando integração com kind"
if ! kind get clusters >/dev/null 2>&1; then
  fail "kind não conseguiu listar clusters. Confira suas credenciais kubeconfig."
fi

log_step "Confirmando entrada dev.local no /etc/hosts"
if ! grep -qE '(^|\s)dev\.local($|\s)' /etc/hosts 2>/dev/null; then
  printf '[preflight] entrada "dev.local" ausente em /etc/hosts. Adicione "127.0.0.1 dev.local" antes de seguir.\n' >&2
fi

log_step "Pré-checks concluídos"

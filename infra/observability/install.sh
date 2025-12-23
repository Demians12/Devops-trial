#!/usr/bin/env bash
set -euo pipefail

log_info() {
  printf '      * %s\n' "$1"
}

run_step() {
  local description="$1"
  shift
  printf '      * %s ... ' "$description"
  local tmp
  tmp=$(mktemp)
  if "$@" >"$tmp" 2>&1; then
    printf 'ok\n'
    if [[ -s "$tmp" ]]; then
      local suppressed=0
      while IFS= read -r line; do
        case "$line" in
          *'Warning: unrecognized format'*|*'spec.SessionAffinity is ignored for headless services'*)
            ((++suppressed))
            ;;
          *'WARNING: This chart is deprecated'*)
            printf '        ! Atenção: %s reportou chart deprecated. Pense em revisar a stack.\n' "$description"
            ;;
          *)
            printf '        ! %s\n' "$line"
            ;;
        esac
      done <"$tmp"
      if (( suppressed > 0 )); then
        printf '        ! %d aviso(s) conhecido(s) suprimido(s)\n' "$suppressed"
      fi
    fi
  else
    printf 'falhou\n'
    sed 's/^/        ! /' "$tmp"
    rm -f "$tmp"
    exit 1
  fi
  rm -f "$tmp"
  return 0
}

ensure_namespace() {
  local ns="$1"
  if ! kubectl get ns "$ns" >/dev/null 2>&1; then
    run_step "Criando namespace '$ns'" kubectl create ns "$ns"
  else
    log_info "Namespace '$ns' disponível"
  fi
}

helm_install() {
  local description="$1"
  shift
  run_step "$description" helm upgrade --install "$@"
}

apply_manifest() {
  local description="$1"
  local namespace="$2"
  local manifest="$3"
  run_step "$description" kubectl apply -n "$namespace" -f "$manifest"
}

INGRESS_TIMEOUT=${HELM_TIMEOUT_INGRESS:-6m}
KPS_TIMEOUT=${HELM_TIMEOUT_KPS:-12m}
LOKI_TIMEOUT=${HELM_TIMEOUT_LOKI:-8m}
TEMPO_TIMEOUT=${HELM_TIMEOUT_TEMPO:-8m}

ensure_namespace observability
ensure_namespace ingress

helm_install "Instalando ingress-nginx" \
  ingress-nginx ingress-nginx/ingress-nginx \
  --namespace ingress \
  --set controller.kind=Deployment \
  --set controller.service.type=NodePort \
  --set controller.service.nodePorts.http=30080 \
  --set controller.service.nodePorts.https=30443 \
  --set controller.watchIngressWithoutClass=true \
  --wait --timeout "${INGRESS_TIMEOUT}" --atomic

helm_install "Instalando kube-prometheus-stack (release kps)" \
  kps prometheus-community/kube-prometheus-stack \
  --namespace observability \
  -f infra/observability/values/kps-values.yaml \
  --wait --timeout "${KPS_TIMEOUT}" --atomic

apply_manifest "Aplicando Loki alert rules (ConfigMap)" observability infra/observability/loki-rules-configmap.yaml

helm_install "Instalando Loki" \
  loki grafana/loki-stack \
  --namespace observability \
  -f infra/observability/values/loki-values.yaml \
  --wait --timeout "${LOKI_TIMEOUT}" --atomic

helm_install "Instalando Tempo" \
  tempo grafana/tempo \
  --namespace observability \
  -f infra/observability/values/tempo-values.yaml \
  --wait --timeout "${TEMPO_TIMEOUT}" --atomic

apply_manifest "Aplicando OTel Collector" observability infra/observability/values/otel-collector.yaml
apply_manifest "Aplicando datasources Grafana" observability infra/observability/grafana-datasources.yaml
apply_manifest "Aplicando dashboards Grafana" observability infra/observability/grafana-dashboards.yaml
apply_manifest "Aplicando Alertmanager" observability infra/observability/alertmanager.yaml
apply_manifest "Aplicando PrometheusRules" observability infra/observability/prometheus-rules.yaml
apply_manifest "Aplicando webhook debug de alertas" observability infra/observability/alert-debugger/deploy.yaml
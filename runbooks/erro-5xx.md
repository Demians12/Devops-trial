# Runbook — Erros 5xx nas APIs Available Schedules

> Objetivo: orientar o diagnóstico e a mitigação de erros 5xx observados nas APIs `available-schedules-python` (v1) e `available-schedules-go` (v2), assim como na UI Node.js (`available-schedules-web`).

## 1. Sinais de detecção

- **Alertas**: verifique o canal/alertmanager para alertas de `% 5xx` ou latência acima do limiar.
- **Dashboards**: abra o dashboard `Available Schedules — Latency & Errors` no Grafana (`http://dev.local/grafana/`). Observe:
  - Painel "Error rate 5xx (%)" — aumento acima de 1% é suspeito.
  - Painel "Percentual de 5xx por rota" — identifica rotas específicas.
  - Painéis de latência p95 (APIs e frontend) — picos podem anteceder falhas.
- **Logs**: Loki (`http://dev.local/grafana/` > Explore > Loki datasource) com consulta `app="available-schedules"` ou filtrando por `service.name`.
- **Traces**: Tempo (`/grafana/` > Explore > Tempo datasource) para rastrear spans com `status.error`.

## 2. Checklist de isolamento rápido

1. **Qual rota?** Veja na tabela de erros por rota (dashboard) ou nos logs.
2. **Impacto:** As duas APIs falham ou apenas uma versão? A UI também retorna 500?
3. **Últimos deploys:** houve `make deploy`, alteração de imagem ou feature flag?
4. **Carga:** rodou teste de carga (k6) recentemente? Consulte Prometheus (`rate(http_requests_total...)`).
5. **Ambiente:** `kubectl get pods -n apps` — há pods reiniciando? Eventos anormais (`kubectl describe pod`).

## 3. Fluxo de diagnóstico detalhado

### 3.1 Backend Python (v1)
- `kubectl logs deploy/available-schedules-python -n apps --tail=200`.
- Verifique logs de exceções em Python; observe mensagens de erro simuladas (decorator com `ERROR_RATE`).
- Verifique métricas em `/metrics` com `curl http://dev.local/v1/metrics` (temporário) para contadores de erro.
- Se necessário, habilite debug local ajustando `ERROR_RATE`/`EXTRA_LATENCY_MS` nos manifests (`infra/apps/available-schedules-python`).

### 3.2 Backend Go (v2)
- `kubectl logs deploy/available-schedules-go -n apps --tail=200`.
- Procure logs JSON com `"status":500`.
- Avalie `ServiceMonitor`/métricas em `http_request_duration_seconds_bucket` para identificar rotas lentas.

### 3.3 Frontend Node.js
- `kubectl logs deploy/available-schedules-web -n apps --tail=200` para falhas de proxy/fetch.
- Conferir Net ingress: `kubectl describe ingress available-schedules -n ingress` (nome fictício; validar). Verifique se backend responde.

### 3.4 Infraestrutura
- Gateway/Ingress: `kubectl get pods -n ingress` e logs do `ingress-nginx-controller`.
- DNS/Serviços: `kubectl get svc -n apps` — endpoints corretos?
- OTel Collector: `kubectl logs deploy/otel-collector -n observability` — falha no export?

## 4. Mitigações rápidas

- **Reiniciar pods** (quando causa for transiente): `kubectl rollout restart deploy/available-schedules-{python,go,web} -n apps`.
- **Escalar réplicas** temporariamente: `kubectl scale deploy/available-schedules-go -n apps --replicas=3`.
- **Desligar injeção de erro**: setar `ERROR_RATE=0` no deployment da API problemática e aplicar `kubectl apply`.
- **Rollback**: se imagem nova, usar `kubectl rollout undo`.

## 5. Pós-incidente

1. Documentar causa-raiz, ações e recomendações.
2. Adicionar alertas/thresholds se faltarem (ex.: alert Prometheus para latência p95 > 500ms).
3. Incluir testes adicionais (ex.: novos cenários no k6) cobrindo rota afetada.
4. Atualizar este runbook conforme aprendizados.

## 6. Artefatos úteis

- **Dashboards**: `Available Schedules — Latency & Errors`, `Kubernetes / Compute Resources / Namespace`
- **Consultas Prometheus**:
  - `sum(rate(http_requests_total{route="/v1/appoints/available-schedule",status=~"5.."}[5m]))`
  - `histogram_quantile(0.95, sum by (route, le)(rate(http_request_duration_seconds_bucket{job=~"available-schedules-(python|go)"}[5m])))`
- **Comandos**:
  - `kubectl get events -n apps --sort-by=.lastTimestamp | tail`
  - `kubectl top pods -n apps`

> Checklist rápido de encerramento: origem identificada ✔ Mitigação aplicada ✔ Erro estabilizado (<1% 5xx) ✔ RCA comunicada ✔ Runbook atualizado ✔

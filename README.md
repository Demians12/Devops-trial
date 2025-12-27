# DevOps / SRE Trial Task — Submission

## Visão geral

Este repositório evolui a infraestrutura base fornecida no desafio da Feegow, com foco em **observabilidade**, **confiabilidade** e **qualidade operacional**.  
A abordagem adotada foi **incremental e orientada a sinais**, validando cada camada (infra → métricas → logs → alertas) antes de avançar para carga, SLOs e automações.

### Principais entregas

- Observabilidade funcional (**Prometheus, Grafana, Loki**)
- Dashboards de aplicação completos (RPS, erros, latência por rota)
- Redução de ruído em alertas
- Alerta baseado em logs (Loki) com correlação por rota
- Alertas provisionados via **IaC** (sem dependência de UI)
- Documentação de decisões e trade-offs operacionais

---

## Setup e validação inicial

### 1. Infraestrutura base

```bash
make up
```

**Validações realizadas:**

- Cluster `kind` criado e reutilizável  
- Ingress funcional em `http://dev.local`  
- Stack de observabilidade ativa (Prometheus, Grafana, Loki, Tempo)  
- Datasources e dashboards provisionados automaticamente no Grafana  
- Alertmanager configurado via arquivo  

---

### 2. Deploy das aplicações

```bash
make deploy
```

**Aplicações disponíveis:**

- **Frontend:** `http://dev.local/`  
- **API v1 (Python):** `/v1/appoints/available-schedule`  
- **API v2 (Go):** `/v2/appoints/available-schedule`  

---

## Ajustes técnicos necessários

### Metrics API (HPA)

Durante a validação inicial, o comando `kubectl top pods` falhava devido à indisponibilidade da **Metrics API**.

**Ação tomada:**

- Correção da instalação do `metrics-server` no cluster

**Resultado:**

- Métricas de CPU e memória disponíveis  
- HPAs funcionando corretamente  
- Base sólida para autoscaling e alertas  

---

### Falha do Loki ao inicializar (Ruler)

O **Loki Ruler** falhava ao inicializar porque o chart monta `/etc/loki` via `Secret` como **read-only**, impedindo a criação do diretório de regras.

**Ação tomada:**

- Ajuste do `ruler.storage` para utilizar `/tmp/loki/rules`  
- Montagem das regras via `ConfigMap` nesse path  
- Separação entre `config.file` e `rule_path`  

**Resultado:**

- Loki inicializa corretamente  
- Ruler ativo e avaliando regras continuamente  
- Alertas de logs totalmente provisionados via IaC  

---

## Observabilidade — Métricas (Prometheus + Grafana)

### Dashboard de aplicação

**Arquivo atualizado:**

- `dashboards/grafana/app-latency.json`

**Painéis implementados:**

- RPS (req/s)  
- Error rate 5xx (%)  
- Latency p95 por rota  
- Percentual de 5xx por rota  

---

## Observabilidade — Logs (Loki)

### Validação de ingestão

```logql
{namespace="apps"}
```

---

### Alerta baseado em logs — `LogsErrorBurst`

```logql
sum by (app, route) (
  count_over_time(
    {namespace="apps"} |= "\" 5"
    | regexp "\"(GET|POST|PUT|DELETE|PATCH) (?P<route>/[^ ]+) HTTP/1\\.1\" (?P<status>5\\d\\d)"
  [5m])
) > 5
```

## CI/CD — Testes unitários

O workflow executa testes unitários de **Python (pytest)** e **Go (go test)** em jobs separados, falhando o pipeline via exit code em caso de erro.  
Isso reduz risco de regressões e acelera feedback, pois valida lógica antes de build/deploy.  
Também é possível reproduzir localmente com `act`, evitando ciclos de push apenas para validar CI.

### Executar CI localmente (act)

```bash
act -j unit-tests-python -P ubuntu-latest=ghcr.io/catthehacker/ubuntu:act-latest
act -j unit-tests-go -P ubuntu-latest=ghcr.io/catthehacker/ubuntu:act-latest
```

### CI local
```bash
cd apps/available-schedules-go && go test ./... -v
cd apps/available-schedules-python && pytest -q

---

## Decisões & trade-offs

- Não usei k6 inicialmente para validar sinais primeiro  
- Corrigi Metrics API antes dos HPAs  
- Alerta baseado em status HTTP e não nível de log  
- Severidade `ticket` para logs  

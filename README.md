# DevOps — Trial Task

Este repositório contém a infraestrutura base do desafio de DevOps. O ambiente traz duas APIs de exemplo (Python e Go) que expõem dados fictícios de agendas médicas. Sua missão é evoluir automação, observabilidade e confiabilidade a partir desse ponto de partida.

## Requisitos

Antes de começar, certifique-se de que você tem as seguintes ferramentas instaladas:

- `docker`
- `kubectl`
- `helm`
- `kind`
- `make`
- `k6` (opcional, para testes de carga)

## Passos

1. **Configuração do Host**: Adicione a seguinte entrada ao seu arquivo `/etc/hosts` para acessar a aplicação localmente:

    ```sh
    127.0.0.1 dev.local
    ```

2. **Subir a Infraestrutura Base**: Crie o cluster Kubernetes local, o ingress controller, as aplicações de exemplo (Python, Go e frontend Node) e a stack de observabilidade.

    > Certifique-se de que o Docker (ou Colima) está em execução e acessível pelo usuário atual antes de rodar este passo. O alvo executa um _preflight_ que checa binários obrigatórios, permissões do Docker e a entrada `dev.local` no `/etc/hosts`.

    ```sh
    make up
    ```

3. **Publicar as Aplicações de Exemplo**: Faça o build, carregue as imagens no cluster e realize o deploy das APIs de agendas (`available-schedules`) em Python, Go e do frontend em Node, incluindo HPA/ServiceMonitor para os serviços de backend.

    ```sh
    make deploy
    ```

4. **Verificar as Aplicações**: Acesse o frontend e os endpoints de agenda para validar o ambiente.
    - Frontend (Node/Tailwind): `http://dev.local/`
    - API v1 (Python/FastAPI): `http://dev.local/v1/appoints/available-schedule`
    - API v2 (Go/net-http): `http://dev.local/v2/appoints/available-schedule`
    - Endpoints de saúde: `http://dev.local/healthz`, `http://dev.local/v2/healthz`
5. **Acessar o Grafana**: Explore os dashboards de monitoramento.
    - **URL**: `http://dev.local/grafana/`
    - **Credenciais**: admin/admin
    - _Nota: Os datasources para Prometheus, Loki e Tempo já estão pré-configurados._
6. **Gerar Carga (Opcional)**: Use o `k6` para gerar carga simultânea nas versões v1/v2 e observar como o cluster reage (autoscaling, alertas, traces).

    ```sh
    make load
    ```

7. **Testar Alertas**: Force a ativação dos alertas para validar a configuração.

    ```sh
    make fire-alerts
    ```

    Para reverter e silenciar os alertas, use `make calm`.

## Intencionalmente "ruim"

Alguns componentes permanecem deliberadamente simples para você evoluir durante o desafio:

- Alertas ainda ruidosos e com limiares frágeis
- HPAs básicos, sem estabilização ou métricas customizadas
- Dashboard de latência com painéis marcados como TODO
- A API em Go expõe métricas, mas ainda não emite traces OTLP (ótimo ponto para explorar)
- O frontend Node entrega uma UI moderna, porém ainda sem testes automatizados ou pipeline dedicado

## Onde mexer

Use estes arquivos como ponto de partida:

- `infra/observability/prometheus-rules.yaml` e `alertmanager.yaml`
- `dashboards/grafana/app-latency.json`
- `infra/apps/available-schedules-python/hpa.yaml` e `infra/apps/available-schedules-go/hpa.yaml`
- `apps/available-schedules-python`, `apps/available-schedules-go` e `apps/available-schedules-web` (instrumentação, UI, flags)

## Desligar

Para parar e limpar todos os recursos do cluster, execute:

```sh
make down
```

## Automação e Diagnóstico

- O alvo `make up` executa `scripts/preflight.sh` antes de qualquer ação. Rode o script diretamente caso queira apenas validar o ambiente.
- Caso o _preflight_ acuse falta de acesso ao Docker, inicie o Docker Desktop/Colima ou ajuste as permissões do socket (`/var/run/docker.sock` ou `$HOME/.docker/run/docker.sock` no modo rootless).
- Se o cluster já existir, o `make up` irá reutilizá-lo automaticamente após garantir que o contexto `kind-devops-lab` está configurado.
- Os `helm upgrade --install` usam `--wait --atomic` com timeouts padrão pensados para clusters rodando em kind. Se quiser acelerar (ou alongar) os aguardos, exporte variáveis como `HELM_TIMEOUT_INGRESS=4m` ou `HELM_TIMEOUT_KPS=10m` antes de rodar o `make up`.
- O `ingress-nginx` expõe HTTP/HTTPS via `NodePort` fixo (30080/30443). O arquivo `infra/kind/cluster.yaml` já faz o _port-forward_ desses NodePorts para a máquina host (80/443), mantendo o acesso via `http://dev.local`.
- O Grafana é servido via Ingress em `http://dev.local/grafana/`; não é necessário executar `kubectl port-forward`.
- As APIs de exemplo ficam acessíveis via ingress: Python em `/v1/appoints/available-schedule` e Go em `/v2/appoints/available-schedule`.
- O datasource Tempo utiliza o serviço exposto na porta `3200`; o `values/kps-values.yaml` já aponta para `tempo.observability.svc.cluster.local:3200`.

## Desafio de Observabilidade (Trilha SRE)

> Use esta trilha para demonstrar suas habilidades em monitoração, automação e qualidade operacional. Siga os itens obrigatórios abaixo; os opcionais rendem bônus.

### Visão geral

- **Prometheus**: refine alertas, crie regras de recording e relacione requisições × latência.
- **Loki**: proponha correlação de logs com traces e dashboards no Explore.
- **Tempo/OTel**: instrumente spans customizados, atributos relevantes e TraceQL para outliers.
- **k6 + Prometheus Remote Write**: combine carga com métricas, analisando efeitos nos painéis/alertas.
- **Checks sintéticos** (Blackbox/Browsertime): avalie uptime da UI e visão fim a fim.

### Entregáveis obrigatórios

1. **Reduzir ruído de alertas** (`infra/observability/prometheus-rules.yaml`)
   - Escolha 2 alertas ruidosos e ajuste thresholds, `for`, labels (`severity`, `service`, `team`) e mensagem (inclua link para runbook).
   - Documente antes/depois + justificativa em 2–4 linhas por alerta.
2. **Definir 1 SLO + burn-rate**
   - Disponibilidade 99,5%/30d para `/api/*` (descreva SLI/SLO).
   - Adicione duas janelas de burn-rate (ex.: 2h/6h) com severidades distintas (page/ticket).
   - Atualize o roteamento no `infra/observability/alertmanager.yaml` para os receivers corretos.
3. **Alerta baseado em logs (Loki)**
   - Criar alerta `LogsErrorBurst` para explosão de `level=error` em 5m.
   - Documentar consulta e rótulos (`app`, `route`, `env`, `version`).
4. **Dashboard de aplicação**
   - Completar `dashboards/grafana/app-latency.json` com RPS, 4xx/5xx, p50/p90/p95 por rota, erros por rota.
5. **CI/CD com testes unitários** (`.github/workflows/ci.yml`)
   - Adicionar estágio “unit tests” (exit code correto, opcional: cobertura/cache).
   - Demonstrar execução local (`act`) ou anexar logs, explicando em poucas linhas ganhos de velocidade/segurança.

### Entregáveis opcionais

- **HPA tuning**: ajustar targets/policies/stabilization e discutir trade-offs.
- **Segurança rápida**: NetworkPolicy no namespace `apps` (deny-all + allow OTel/Grafana) e `trivy` na imagem, corrigindo 1 achado.
- **Datadog (sem conta real)**: produzir 2 JSONs de Monitors (`HighErrorRate`, `HighLatencyP95`) inspirados no provider Terraform.
- **Runbook extra**: novo runbook “Latência p95 alta” com checagens e rollback.

### Checklist de submissão

- Branch/PRs com diffs dos alertas, dashboards, pipelines etc.
- Pastas atualizadas:
  - `dashboards/` (JSON exportados)
  - `observability/` (rules, alertmanager, datasources, dashboards)
  - `runbooks/` (1-pager HighErrorRate + extras opcionais)
  - `datadog-monitors/` (opcional)
- Registre antes/depois dos alertas e resumo dos ajustes.

### Recursos úteis

- Grafana, Prometheus, Loki, Tempo docs.
- Documentação do provedor [Datadog Terraform](https://registry.terraform.io/providers/DataDog/datadog/latest/docs) para monitores.
- Comandos rápidos: `make up`, `make deploy`, `make load`, `make fire-alerts`, `kubectl logs -n apps`, `k6 run tests/k6/available_schedules.js`.

> Dica: mantenha anotações das decisões e cite-as nos PRs/runbooks; isso ajuda na avaliação do raciocínio operacional.

# DevOps Senior — Trial Task

Este repositório contém a infraestrutura base do desafio de DevOps Senior. O ambiente traz duas APIs de exemplo (Python e Go) que expõem dados fictícios de agendas médicas, inspirados no endpoint `GET /appoints/available-schedule` da Feegow. Sua missão é evoluir automação, observabilidade e confiabilidade a partir desse ponto de partida.

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

2. **Subir a Infraestrutura Base**: Crie o cluster Kubernetes local, o ingress controller e a stack de observabilidade.

    > Certifique-se de que o Docker (ou Colima) está em execução e acessível pelo usuário atual antes de rodar este passo. O alvo executa um _preflight_ que checa binários obrigatórios, permissões do Docker e a entrada `dev.local` no `/etc/hosts`.

    ```sh
    make up
    ```

3. **Publicar as Aplicações de Exemplo**: Faça o build, carregue as imagens no cluster e realize o deploy das APIs de agendas (`available-schedules`) em Python e Go, incluindo HPA e ServiceMonitor.

    ```sh
    make deploy
    ```

4. **Verificar as Aplicações**: Acesse os endpoints de health check e de agenda disponível para validar que tudo está ativo.
    - Python: `http://dev.local/healthz` e `http://dev.local/appoints/available-schedule`
    - Go: `http://dev.local/go/healthz` e `http://dev.local/go/appoints/available-schedule`
    - As rotas principais também informam caminhos úteis: `http://dev.local/` e `http://dev.local/go`
5. **Acessar o Grafana**: Explore os dashboards de monitoramento.
    - **URL**: `http://dev.local/grafana/`
    - **Credenciais**: admin/admin
    - _Nota: Os datasources para Prometheus, Loki e Tempo já estão pré-configurados._
6. **Gerar Carga (Opcional)**: Use o `k6` para gerar carga simultânea em ambas as APIs e observar o comportamento da infraestrutura.

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

## Onde mexer

Use estes arquivos como ponto de partida:

- `infra/observability/prometheus-rules.yaml` e `alertmanager.yaml`
- `dashboards/grafana/app-latency.json`
- `infra/apps/available-schedules-python/hpa.yaml` e `infra/apps/available-schedules-go/hpa.yaml`
- `apps/available-schedules-python` e `apps/available-schedules-go` (instrumentação/flags)

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
- As APIs de exemplo ficam acessíveis via ingress: Python em `/appoints/available-schedule` e Go em `/go/appoints/available-schedule`.
- O datasource Tempo utiliza o serviço exposto na porta `3200`; o `values/kps-values.yaml` já aponta para `tempo.observability.svc.cluster.local:3200`.

# DevOps Senior — Trial Task

Este é um desafio para a posição de DevOps Senior. As seções abaixo guiarão você pela configuração e pelos objetivos do projeto.

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

2. **Subir o Ambiente**: Crie o cluster Kubernetes local, o ingress controller e a stack de observabilidade.

    > Certifique-se de que o Docker (ou Colima) está em execução e acessível pelo usuário atual antes de rodar este passo. O alvo executa um _preflight_ que checa binários obrigatórios, permissões do Docker e a entrada `dev.local` no `/etc/hosts`.

    ```sh
    make up
    ```

3. **Deploy da Aplicação**: Faça o build, carregue a imagem no cluster e realize o deploy da `orders-api`, incluindo o HPA e o ServiceMonitor.

    ```sh
    make deploy
    ```

4. **Verificar a Aplicação**: Acesse o endpoint de health check para confirmar que a aplicação está no ar.
    - `http://dev.local/healthz`
    - A página principal (`http://dev.local/`) retorna um JSON com o status do ambiente e os endpoints principais.
5. **Acessar o Grafana**: Explore os dashboards de monitoramento.
    - **URL**: `http://dev.local/grafana/`
    - **Credenciais**: admin/admin
    - _Nota: Os datasources para Prometheus, Loki e Tempo já estão pré-configurados._
6. **Gerar Carga (Opcional)**: Use o `k6` para gerar carga na aplicação e observar seu comportamento.

    ```sh
    make load
    ```

7. **Testar Alertas**: Force a ativação dos alertas para validar a configuração.

    ```sh
    make fire-alerts
    ```

    Para reverter e silenciar os alertas, use `make calm`.

## Intencionalmente "ruim"

Para este desafio, alguns componentes foram configurados de forma subótima intencionalmente. Seu objetivo é melhorá-los.

- Alertas ruidosos
- HPA subótimo
- Painel incompleto

## Onde mexer

Os arquivos a seguir são os pontos de partida para suas alterações:

- `infra/observability/prometheus-rules.yaml` e `alertmanager.yaml`
- `dashboards/grafana/app-latency.json`
- `infra/apps/orders-api/hpa.yaml`
- `apps/orders-api` (instrumentação/flags)

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

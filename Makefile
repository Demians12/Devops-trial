SHELL := /bin/bash
KIND_CLUSTER := devops-lab
APPS := available-schedules-python available-schedules-go available-schedules-web
BACKEND_APPS := available-schedules-python available-schedules-go
NAMESPACE_APPS := apps
NAMESPACE_OBS := observability

log = @sh -c 'printf "\n==> %s\n" "$$1"' _ "$(1)"
sublog = @sh -c 'printf "    - %s\n" "$$1"' _ "$(1)"

.PHONY: up down deploy app.build app.load helm.repos obs.install load fire-alerts calm dashboards.export

up: ## Cria cluster kind e instala stack de observabilidade + ingress
	$(call log,Validação de pré-requisitos)
	@./scripts/preflight.sh
	$(call log,Garantindo cluster kind '$(KIND_CLUSTER)')
	@if ! kind get clusters 2>/dev/null | grep -q '^$(KIND_CLUSTER)$$'; then \
		kind create cluster --name $(KIND_CLUSTER) --config infra/kind/cluster.yaml >/dev/null; \
		printf '    - cluster criado\n'; \
	else \
		printf '    - cluster já existente, reutilizando\n'; \
	fi
	@kind export kubeconfig --name $(KIND_CLUSTER) >/dev/null
	@kubectl config use-context kind-$(KIND_CLUSTER) >/dev/null
	$(call sublog,Aguardando nós prontos)
	@kubectl wait --for=condition=Ready nodes --all --timeout=120s >/dev/null
	$(call sublog,Nós prontos)
	$(call log,Atualizando repositórios Helm)
	@./infra/observability/repos.sh
	$(call log,Instalando stack de observabilidade)
	@./infra/observability/install.sh
	$(call sublog,Aguardando ingress controller)
	@kubectl wait --namespace ingress --for=condition=ready pod --selector=app.kubernetes.io/component=controller --timeout=120s >/dev/null
	$(call sublog,Ingress controller pronto)
	$(call sublog,Aguardando webhook do ingress)
	@sleep 20
	$(call sublog,Webhook do ingress pronto)
	$(call log,Preparando namespace de aplicações e ingressos)
	@if ! kubectl get ns $(NAMESPACE_APPS) >/dev/null 2>&1; then \
		kubectl create ns $(NAMESPACE_APPS) >/dev/null; \
		printf '    - namespace %s criado\n' "$(NAMESPACE_APPS)"; \
	else \
		printf '    - namespace %s já existe\n' "$(NAMESPACE_APPS)"; \
	fi
	@kubectl apply -f infra/ingress/ >/dev/null
	$(call sublog,Ingressos aplicados)

app.build: ## Build das aplicações exemplo (Python e Go)
	$(call log,Build das imagens de exemplo)
	@for app in $(APPS); do \
		printf '    - construindo %s\n' $$app; \
		docker build -t $$app:local ./apps/$$app >/dev/null; \
	done
	$(call sublog,Imagens construídas)

app.load: app.build ## Carrega imagens no kind
	$(call log,Carregando imagens no cluster kind)
	@for app in $(APPS); do \
		printf '    - carregando %s\n' $$app; \
		kind load docker-image $$app:local --name $(KIND_CLUSTER) >/dev/null; \
	done
	$(call sublog,Imagens carregadas)

deploy: app.load ## Deploy das aplicações, HPA e ServiceMonitor
	$(call log,Aplicando manifests das aplicações)
	@for app in $(APPS); do \
		printf '    - aplicando manifests de %s\n' $$app; \
		kubectl apply -n $(NAMESPACE_APPS) -f infra/apps/$$app/deployment.yaml >/dev/null; \
		kubectl apply -n $(NAMESPACE_APPS) -f infra/apps/$$app/service.yaml >/dev/null; \
		if [ -f infra/apps/$$app/hpa.yaml ]; then \
			kubectl apply -n $(NAMESPACE_APPS) -f infra/apps/$$app/hpa.yaml >/dev/null; \
		fi; \
		if [ -f infra/apps/$$app/servicemonitor.yaml ]; then \
			kubectl apply -n $(NAMESPACE_APPS) -f infra/apps/$$app/servicemonitor.yaml >/dev/null; \
		fi; \
	done
	$(call sublog,Recursos aplicados)

load: ## Gera carga para acionar alertas
	$(call log,Executando teste de carga (k6))
	@k6 run tests/k6/available_schedules.js || echo "Instale k6 para rodar este alvo"

fire-alerts: ## Aumenta erro/latência por 15m
	$(call log,Ativando flags de erro/latência)
	@for app in $(BACKEND_APPS); do \
		kubectl -n $(NAMESPACE_APPS) set env deploy/$$app ERROR_RATE=0.10 EXTRA_LATENCY_MS=400 >/dev/null || true; \
	done
	@echo "Mantendo flags ligadas por ~15m; depois rode 'make calm' para normalizar"

calm: ## Normaliza flags
	$(call log,Normalizando flags das aplicações)
	@for app in $(BACKEND_APPS); do \
		kubectl -n $(NAMESPACE_APPS) set env deploy/$$app ERROR_RATE=0.01 EXTRA_LATENCY_MS=0 >/dev/null || true; \
	done

obs.install: ## (re)instala componentes de observabilidade
	$(call log,Reinstalando stack de observabilidade)
	@./infra/observability/install.sh

dashboards.export:
	$(call log,Exportando dashboards)
	@echo "Exporte via UI do Grafana e salve em dashboards/grafana/*.json"

down: ## Destroi cluster
	$(call log,Removendo cluster kind '$(KIND_CLUSTER)')
	@kind delete cluster --name $(KIND_CLUSTER) >/dev/null
	$(call sublog,Cluster removido)
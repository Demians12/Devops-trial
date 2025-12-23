# Runbook — HighErrorRate

## Descrição

O alerta **HighErrorRate** monitora a **taxa de respostas HTTP 5xx**
em relação ao total de requisições atendidas pela aplicação.

Ele é baseado em **métricas Prometheus** e representa um sinal de
**degradação real da confiabilidade do serviço**.

Este alerta é considerado **crítico** porque pode indicar violação
iminente ou em andamento do SLO de disponibilidade.

---

## Quando este alerta dispara

- A proporção de respostas HTTP **5xx** ultrapassa o threshold definido
- Avaliação em janela contínua (rolling window)
- Uso de `for` para evitar flapping causado por picos transitórios

**Severidade:** `page`

---

## Impacto esperado

- Usuários finais estão recebendo erros de forma consistente
- Experiência degradada ou indisponibilidade parcial do serviço
- Risco elevado de violação de SLO (ex.: disponibilidade mensal)

---

## Como investigar (checklist)

1. **Identificar serviço e rota afetados**
   - Ver labels do alerta (`service`, `route`, `job`)

2. **Correlacionar métricas no Grafana**
   - RPS (houve aumento repentino?)
   - Latência p95/p99 (há degradação?)
   - Distribuição de status 4xx vs 5xx

3. **Verificar mudanças recentes**
   - Deploys ou rollouts recentes
   - Alterações de configuração
   - Incidentes em dependências externas

4. **Validar estado do cluster**
   - Pods reiniciando?
   - HPA atingindo limites?
   - Saturação de CPU/memória?

---

## Ações imediatas de mitigação

- Se associado a deploy recente:
  - Executar **rollback** da versão
- Se associado a carga:
  - Escalar horizontalmente (HPA)
  - Avaliar limitação de tráfego
- Se erro externo:
  - Aplicar circuit breaker ou fallback (quando aplicável)

---

## Pós-incidente / melhorias

- Avaliar se o threshold está adequado ao perfil real da aplicação
- Refinar alertas preditivos (ex.: logs ou latência)
- Criar alerta de **burn-rate** para antecipar violações de SLO
- Atualizar este runbook com aprendizados do incidente

---

## Observações

Este alerta foi intencionalmente ajustado para:

- Reduzir ruído (evitar falsos positivos)
- Disparar apenas quando há impacto real ao usuário
- Ser acionável por um engenheiro em regime de plantão

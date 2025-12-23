# Runbook — LogsErrorBurst

## O que é este alerta

O alerta **LogsErrorBurst** detecta um aumento abrupto de respostas HTTP **5xx**
em uma janela curta (5 minutos), com base em **logs de acesso** via Loki.

Ele é útil para identificar falhas **antes** que métricas agregadas
(SLI/SLO) reflitam o problema.

---

## Quando este alerta dispara

- Mais de **5 respostas 5xx** em **5 minutos**
- Avaliado pelo **Loki Ruler**
- Agregado por `app` e `route`

Severidade: **ticket**

---

## Impacto esperado

- Usuários podem estar recebendo erros intermitentes
- Possível degradação parcial do serviço
- Ainda não caracteriza violação de SLO

---

## Como investigar (checklist)

1. Identificar rota afetada no alerta (`route`)
2. Consultar logs recentes no Loki:
   ```logql
   {namespace="apps", route="<rota>"} |= "5"

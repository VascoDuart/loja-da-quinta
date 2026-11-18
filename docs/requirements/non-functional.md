# Requisitos não funcionais

Cada requisito tem uma forma de ser verificado. Os que se podem automatizar correm no CI.

| ID | Categoria | Requisito | Como se verifica |
|---|---|---|---|
| RNF-01 | Segurança | Cumpre o OWASP ASVS 4.0, nível 1. | [Checklist](../security/asvs-checklist.md), revista em cada release |
| RNF-02 | Segurança | Sem vulnerabilidades conhecidas críticas ou altas em dependências e imagens. | govulncheck, npm audit e Trivy bloqueiam o PR |
| RNF-03 | Desempenho | p95 abaixo de 300 ms nos endpoints da API com a instância acordada. | Teste de carga k6 contra staging |
| RNF-04 | Disponibilidade | Arranque a frio até 60 s aceite (plano gratuito). O frontend mostra “a acordar o servidor” em vez de um erro. | Teste E2E com a API parada |
| RNF-05 | Usabilidade | Uma venda com 3 produtos regista-se em menos de 30 s. | Teste manual cronometrado |
| RNF-06 | Acessibilidade | WCAG 2.1 AA nas páginas principais. | axe-core nos testes Playwright |
| RNF-07 | Responsivo | Funciona de 360 px a 1440 px; ecrã de venda otimizado para tablet. | Playwright em 3 tamanhos de ecrã |
| RNF-08 | Manutenção | Cobertura de pelo menos 80% nos pacotes de domínio e aplicação; lint sem avisos. | `go test -cover`, golangci-lint, ESLint no CI |
| RNF-09 | Testabilidade | Testes de integração com PostgreSQL real; pipeline completo em menos de 10 minutos. | testcontainers-go; duração do CI |
| RNF-10 | Observabilidade | Logs JSON com request ID; endpoints `/healthz` e `/readyz`. | Teste de integração dos endpoints |
| RNF-11 | Integridade | Vendas e ajustes são transacionais; o stock nunca fica negativo. | Restrição `CHECK` na BD e teste de concorrência |
| RNF-12 | Recuperação | Backup semanal com `pg_dump`, cifrado com age, guardado 30 dias. | Workflow agendado e restauro testado uma vez por mês |
| RNF-13 | Privacidade | RGPD: só dados mínimos dos funcionários; IPs da auditoria apagados ao fim de 90 dias. | Job de limpeza com teste |
| RNF-14 | Custo | 0 € por mês. | Só planos gratuitos ([ADR-0008](../adr/0008-alojamento-gratuito.md)) |
| RNF-15 | Portabilidade | Tudo arranca localmente com `docker compose up`. | Passo do CI que levanta o compose |
| RNF-16 | Localização | pt-PT, euro e fuso Europe/Lisbon na interface; datas guardadas em UTC. | Testes de formatação e de mudança de hora |

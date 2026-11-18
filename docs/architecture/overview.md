# Visão geral da arquitetura

A arquitetura segue o modelo [C4](https://c4model.com/). Os níveis 1 e 2 estão aqui; o nível 3 está em [backend.md](backend.md) e [frontend.md](frontend.md).

## C4 nível 1: contexto

```mermaid
flowchart LR
    V["👤 Visitante<br/><small>pessoa da zona</small>"]
    F["👤 Funcionário<br/><small>atende ao balcão</small>"]
    G["👤 Gestor<br/><small>dono da quinta</small>"]
    S["<b>Loja da Quinta</b><br/><small>catálogo · stock · vendas<br/>relatórios · utilizadores</small>"]
    R["Resend<br/><small>serviço de e-mail externo</small>"]

    V -- "consulta o catálogo" --> S
    F -- "regista stock e vendas" --> S
    G -- "gere e analisa" --> S
    S -- "pede envio" --> R
    R -- "e-mail diário de stock baixo e validade" --> G

    classDef system fill:#e1eee0,stroke:#2e6a3b,stroke-width:2px,color:#1b221e
    classDef ext stroke-dasharray:5 4
    class S system
    class R ext
```

O MVP só depende de um sistema externo: o envio de e-mail. Não há pagamentos nem integração fiscal, o que reduz a superfície de ataque.

## C4 nível 2: contentores e alojamento

```mermaid
flowchart LR
    subgraph B["Browser"]
        SPA["SPA React + TS<br/><small>token só em memória</small>"]
    end
    subgraph V["Vercel · Hobby"]
        CDN["CDN estática<br/><small>rewrite /api/* para a API</small>"]
    end
    subgraph R["Render · Free"]
        API["<b>API Go</b><br/><small>contentor Docker</small>"]
        JOB["Job de alertas<br/><small>mesmo processo · diário</small>"]
    end
    subgraph N["Neon · Free"]
        PG[("PostgreSQL 16<br/><small>TLS obrigatório</small>")]
    end
    subgraph E["Resend · Free"]
        MAIL["API de e-mail"]
    end
    subgraph GH["GitHub"]
        CI["GitHub Actions<br/><small>lint · testes · scans · build · migrações · deploy</small>"]
    end

    SPA -- "HTTPS" --> CDN
    CDN -- "proxy /api/*" --> API
    API -- "pgx · TLS" --> PG
    JOB -- "HTTPS" --> MAIL
    CI -. "deploy da SPA" .-> CDN
    CI -. "deploy da imagem" .-> API
    CI -. "migrações goose" .-> PG

    classDef main fill:#e1eee0,stroke:#2e6a3b,stroke-width:2px,color:#1b221e
    class API main
```

O browser só fala com a Vercel. O rewrite de `/api/*` põe a API na **mesma origem** ([ADR-0007](../adr/0007-mesma-origem-via-rewrite.md)): não é preciso CORS e o cookie `SameSite=Strict` funciona.

> **Risco a validar logo no início:** confirmar que o tempo limite do proxy da Vercel aguenta o arranque a frio da Render. Se não aguentar, a alternativa é um domínio próprio partilhado pelos dois serviços.

## Ambientes

| Ambiente | Origem | Frontend | API | Base de dados |
|---|---|---|---|---|
| Local | máquina do programador | Vite dev server | `go run` ou Docker | PostgreSQL no `docker compose` |
| Staging | branch `dev` | deploy do branch `dev` na Vercel | serviço gratuito na Render | branch `staging` da Neon |
| Produção | branch `main` | Vercel (produção) | serviço gratuito na Render | branch principal da Neon |

# gogather

Monitora reviews da **Apple App Store** e **Google Play Store** e envia notificações para o **Slack** em tempo real, sem duplicatas.

---

## Sumário

- [Como funciona](#como-funciona)
- [Mensagens no Slack](#mensagens-no-slack)
- [Configuração](#configuração)
- [Deploy](#deploy)
  - [GitHub Actions](#github-actions-recomendado)
  - [Docker / VPS](#docker--vps)
  - [Fly.io](#flyio)
- [Variáveis de ambiente](#variáveis-de-ambiente)
- [Estrutura do projeto](#estrutura-do-projeto)
- [Detalhes técnicos](#detalhes-técnicos)

---

## Como funciona

```
 Apple App Store          Google Play Store
 (RSS + Lookup API)       (batchexecute RPC)
        │                        │
        └───────────┬────────────┘
                    │
                    ▼
          ┌──────────────────┐
          │  Filtro de idade │  descarta reviews antigas
          └────────┬─────────┘
                    │
                    ▼
          ┌──────────────────┐
          │ seen_reviews.json│  filtra IDs já enviados
          └────────┬─────────┘
                   │ somente reviews novas
                   ▼
          ┌──────────────────┐
          │  Slack Webhook   │
          └──────────────────┘
```

A cada poll o serviço busca as reviews de cada loja ativa, descarta as mais antigas que `MAX_REVIEW_AGE_DAYS`, ignora as que já foram enviadas (IDs persistidos em disco) e dispara uma mensagem no Slack para cada review nova, em ordem cronológica.

---

## Mensagens no Slack

Cada review chega como uma mensagem individual com barra colorida indicando a nota:

```
│ 🟢  *Dafiti: Shopping no seu Bolso*   ★★★★★
│     :applestore: App Store
│
│     > Amo o app! Produtos de qualidade e preços acessíveis.
│     > Uma ressalva para o parcelamento, antes era em 10x...
│
│     👤 Katia Botion   ·   📅 05/12/2025   ·   📱 v18.3.9
```

| Elemento | Descrição |
|---|---|
| Barra lateral | 🟢 Verde = 4-5★ · 🟡 Amarelo = 3★ · 🔴 Vermelho = 1-2★ |
| Cabeçalho | Nome do app em negrito + estrelas unicode |
| Loja | Ícone `:applestore:` ou `:google_play:` + nome da loja |
| Corpo | Texto da review em bloco de citação |
| Título | Exibido em negrito antes do corpo, quando presente |
| Rodapé | Autor · Data · Versão do app no momento da review |

---

## Configuração

Copie `.env.example` para `.env`:

```bash
cp .env.example .env
```

```env
# ── Obrigatório ────────────────────────────────────────────────
SLACK_WEBHOOK_URL=https://hooks.slack.com/services/XXX/YYY/ZZZ

# ── Comportamento ──────────────────────────────────────────────
POLL_INTERVAL_SECONDS=3600   # 0 = executar uma vez e sair
MAX_REVIEW_AGE_DAYS=90       # ignora reviews mais antigas que N dias

# ── Apple App Store ────────────────────────────────────────────
APPLE_ENABLED=true
APPLE_APP_ID=564924168       # ID numérico da URL do app
APPLE_REGION=br              # br · us · pt · es …

# ── Google Play Store ──────────────────────────────────────────
PLAY_STORE_ENABLED=true
PLAY_STORE_PACKAGE=br.com.dafiti   # package name do app
PLAY_STORE_LANG=pt                 # pt · en · es …
PLAY_STORE_COUNTRY=br              # br · us …
```

### Monitorar apenas uma loja

Use `APPLE_ENABLED` e `PLAY_STORE_ENABLED` para ligar/desligar cada plataforma sem precisar apagar as variáveis de configuração:

```env
# Só Android
APPLE_ENABLED=false
PLAY_STORE_ENABLED=true

# Só iOS
APPLE_ENABLED=true
PLAY_STORE_ENABLED=false
```

Os valores aceitos para as flags são: `true` / `false` · `1` / `0` · `yes` / (qualquer outro valor).

### Onde encontrar o App ID da Apple

```
https://apps.apple.com/br/app/nome-do-app/id564924168
                                               ^^^^^^^^^ ← APPLE_APP_ID
```

### Onde encontrar o Slack Webhook URL

1. Acesse [api.slack.com/apps](https://api.slack.com/apps) → crie ou selecione um app
2. **Incoming Webhooks** → ative → **Add New Webhook to Workspace**
3. Escolha o canal → copie a URL gerada

---

## Deploy

### GitHub Actions (recomendado)

A opção mais simples: zero infraestrutura, gratuita dentro dos limites do plano free (2.000 min/mês — cada execução leva ~10s).

O workflow já está incluído em `.github/workflows/poll.yml` e roda a cada hora. Para ativar:

**1. Suba o repositório para o GitHub**

**2. Adicione as variáveis em `Settings → Secrets and variables → Actions`:**

| Tipo | Nome | Valor |
|---|---|---|
| **Secret** | `SLACK_WEBHOOK_URL` | URL do webhook (oculta nos logs) |
| Variable | `APPLE_ENABLED` | `true` ou `false` |
| Variable | `APPLE_APP_ID` | ID numérico do app |
| Variable | `APPLE_REGION` | `br` |
| Variable | `PLAY_STORE_ENABLED` | `true` ou `false` |
| Variable | `PLAY_STORE_PACKAGE` | package name |
| Variable | `PLAY_STORE_LANG` | `pt` |
| Variable | `PLAY_STORE_COUNTRY` | `br` |
| Variable | `MAX_REVIEW_AGE_DAYS` | `90` |

**3. Ative o workflow em `Actions → Poll Reviews → Enable`**

Para disparar manualmente: `Actions → Poll Reviews → Run workflow`.

> **Estado entre execuções:** o `seen_reviews.json` é persistido via `actions/cache`. Se o cache for eviccionado (GitHub mantém por 7 dias), o pior caso é uma leva de reviews dentro da janela `MAX_REVIEW_AGE_DAYS` chegar no Slack uma única vez.

---

### Docker / VPS

Indicado para quem quer controle total ou já possui um servidor. Hetzner (€3,29/mês) e DigitalOcean ($4/mês) são boas opções.

```bash
# no servidor
git clone https://github.com/hlfdev/gogather
cd gogather
cp .env.example .env && nano .env   # preencha as variáveis
```

```yaml
# docker-compose.yml
services:
  gogather:
    build: .
    env_file: .env
    volumes:
      - ./data:/app/data
    restart: unless-stopped
```

```bash
docker compose up -d
docker compose logs -f
```

O `seen_reviews.json` fica em `./data/` e persiste entre reinicializações.

> Para rodar sem Docker: `export $(grep -v '^#' .env | xargs) && go run ./cmd`

---

### Fly.io

Plataforma com free tier que suporta processos sempre ativos e volumes persistentes.

```bash
fly launch          # detecta o Dockerfile automaticamente
fly volumes create gogather_data --size 1
fly secrets set SLACK_WEBHOOK_URL=... APPLE_APP_ID=... # demais variáveis
fly deploy
```

No `fly.toml` gerado, adicione:

```toml
[mounts]
  source = "gogather_data"
  destination = "/app/data"
```

---

## Variáveis de ambiente

| Variável | Obrigatória | Padrão | Descrição |
|---|---|---|---|
| `SLACK_WEBHOOK_URL` | ✅ | — | URL do Incoming Webhook do Slack |
| `POLL_INTERVAL_SECONDS` | ❌ | `3600` | Segundos entre cada poll. `0` = executar uma vez e sair |
| `MAX_REVIEW_AGE_DAYS` | ❌ | `90` | Ignora reviews mais antigas que N dias. `0` = sem filtro |
| `APPLE_ENABLED` | ❌ | `true` | `false` desativa a App Store sem apagar as demais vars |
| `APPLE_APP_ID` | ❌ | — | ID numérico do app na App Store |
| `APPLE_REGION` | ❌ | `br` | Região da App Store |
| `PLAY_STORE_ENABLED` | ❌ | `true` | `false` desativa o Play Store sem apagar as demais vars |
| `PLAY_STORE_PACKAGE` | ❌ | — | Package name do app no Play Store |
| `PLAY_STORE_LANG` | ❌ | `pt` | Idioma das reviews |
| `PLAY_STORE_COUNTRY` | ❌ | `br` | País das reviews |

---

## Estrutura do projeto

```
gogather/
├── .github/
│   └── workflows/
│       └── poll.yml            # Workflow do GitHub Actions (cron horário)
├── cmd/
│   └── main.go                 # Entrypoint: loop de polling ou execução única
├── config/
│   └── config.go               # Lê e valida variáveis de ambiente
├── internal/
│   ├── scraper/
│   │   ├── review.go           # Struct Review e tipo Source (Apple / Play Store)
│   │   ├── apple.go            # Scraper da Apple App Store
│   │   ├── playstore.go        # Scraper do Google Play Store (batchexecute RPC)
│   │   └── playstore_parser.go # Parser da resposta batchexecute
│   ├── notifier/
│   │   └── slack.go            # Monta e envia mensagens via Slack Webhook
│   └── store/
│       └── store.go            # Persiste IDs já enviados em seen_reviews.json
├── .env.example
├── Dockerfile
└── go.mod
```

---

## Detalhes técnicos

### Apple App Store

O nome do app é obtido via **iTunes Lookup API** (chamada única, resultado cacheado):

```
GET https://itunes.apple.com/lookup?id={appID}&country={region}
→ results[0].trackName
```

As reviews são buscadas no **feed RSS/JSON público** da Apple (sem autenticação):

```
GET https://itunes.apple.com/{region}/rss/customerreviews/page={n}/id={appID}/sortby=mostrecent/json
```

- Páginas 1 e 2 → até **100 reviews** por poll, ordenadas da mais recente para a mais antiga

### Google Play Store

As reviews são obtidas via **endpoint interno `batchexecute`** do Play Store — o mesmo que o site usa internamente. Não requer autenticação nem API key.

```
POST https://play.google.com/_/PlayStoreUi/data/batchexecute?hl={lang}&gl={country}
RPC: oCPfdb  |  sort=2 (mais recentes primeiro)
```

- Retorna até **100 reviews** por requisição, ordenadas da mais recente para a mais antiga
- Sem autenticação ou API key
- O nome do app é extraído da meta tag `og:title` da página do Play Store

### Deduplicação

O arquivo `seen_reviews.json` mantém um array com os IDs de todas as reviews já enviadas. A cada poll:

1. Reviews com ID já presente são ignoradas
2. Reviews novas são enviadas ao Slack em ordem cronológica (mais antiga → mais recente)
3. O arquivo é atualizado imediatamente após cada envio bem-sucedido

Isso garante que reinicializações do serviço não causem reenvios.

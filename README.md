# agentguard — Trust & Safety Infrastructure for AI Agents

> El escudo que tus agentes necesitan. Cumplimiento, seguridad y observabilidad en una API.

[![CI/CD](https://github.com/belentani7/agentguard/actions/workflows/ci.yml/badge.svg)](https://github.com/belentani7/agentguard/actions/workflows/ci.yml)
[![Security](https://github.com/belentani7/agentguard/actions/workflows/ci.yml/badge.svg?branch=main&job=security-scan)](https://github.com/belentani7/agentguard/actions/workflows/ci.yml)
[![Docker](https://img.shields.io/badge/docker-ghcr.io%2Fbelentani7%2Fagentguard-blue)](https://github.com/belentani7/agentguard/pkgs/container/agentguard)

---

## ¿Qué es agentguard?

agentguard es la capa de **Trust & Safety** para agentes IA en producción. No es un wrapper — es infraestructura.

```
┌─────────────────────────────────────────────────────────────┐
│  TU AGENTE IA                                               │
└─────────────────┬───────────────────────────────────────────┘
                  ▼
┌─────────────────────────────────────────────────────────────┐
│  agentguard (Edge Runtime)                                  │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐           │
│  │ Guardrails  │ │  Auditoría  │ │  Observab.  │           │
│  │  Políticas  │ │  Inmutable  │ │  Métricas   │           │
│  └─────────────┘ └─────────────┘ └─────────────┘           │
└─────────────────────────────────────────────────────────────┘
                  ▼
┌─────────────────────────────────────────────────────────────┐
│  PRODUCCIÓN SEGURA                                          │
└─────────────────────────────────────────────────────────────┘
```

---

## Características

| Módulo | Qué hace |
|--------|----------|
| **Guardrails** | Políticas declarativas versionadas. Bloquea PII, comandos shell, accesos no autorizados, jailbreaks. |
| **Auditoría** | Hash chain inmutable. Cada decisión firmada. Exporta evidencias para EU AI Act, SOC2, GDPR. |
| **Observabilidad** | Prometheus, OpenTelemetry, logs estructurados. Dashboards Grafana listos. |
| **Secretos** | Vault integrado. Rotación automática. Just-in-time access con aprobación. |
| **Cumplimiento** | EU AI Act, GDPR, SOC2, ISO 27001. Reportes automáticos. |
| **Latencia** | <50ms P99. Edge runtime distribuido. No ralentiza a tus agentes. |

---

## Inicio rápido

### Prerrequisitos
- Docker 24+ / Docker Compose 2.20+
- Cuenta Stripe (para billing)
- PostgreSQL 16+ (incluido en compose)

### 1. Clonar y configurar
```bash
git clone https://github.com/belentani7/agentguard.git
cd agentguard
cp .env.example .env
# Edita .env con tus claves Stripe, SECRET_KEY, etc.
```

### 2. Levantar stack completo
```bash
docker compose up -d
```

La API estará en `http://localhost:8000`  
Docs interactivas: `http://localhost:8000/docs`  
Health check: `http://localhost:8000/api/v1/health`

### 3. Ver logs
```bash
docker compose logs -f api
```

---

## Variables de entorno (.env)

```bash
# Database (docker-compose usa estos valores)
POSTGRES_DB=agentguard
POSTGRES_USER=agentguard
POSTGRES_PASSWORD=tu_password_seguro_aqui

# Security
SECRET_KEY=genera_con_openssl_rand_base64_32
ALGORITHM=HS256
ACCESS_TOKEN_EXPIRE_MINUTES=30

# Stripe (obtén en dashboard.stripe.com)
STRIPE_SECRET_KEY=sk_test_...
STRIPE_PUBLISHABLE_KEY=pk_test_...
STRIPE_WEBHOOK_SECRET=whsec_...
STRIPE_PRICE_ID_MONTHLY=price_...
STRIPE_PRICE_ID_YEARLY=price_...
STRIPE_PRICE_ID_ENTERPRISE=price_...

# Frontend
FRONTEND_URL=http://localhost:3000
ENVIRONMENT=development
```

### Generar SECRET_KEY
```bash
openssl rand -base64 32
```

---

## API Endpoints

### Auth
| Método | Endpoint | Descripción |
|--------|----------|-------------|
| POST | `/api/v1/auth/register` | Registrar usuario |
| POST | `/api/v1/auth/login` | Login + JWT |
| GET | `/api/v1/auth/me` | Usuario actual |
| PATCH | `/api/v1/auth/me` | Actualizar perfil |

### Organizaciones
| Método | Endpoint | Descripción |
|--------|----------|-------------|
| POST | `/api/v1/organizations` | Crear org |
| GET | `/api/v1/organizations/me` | Mi organización |

### Billing (Stripe)
| Método | Endpoint | Descripción |
|--------|----------|-------------|
| POST | `/api/v1/billing/checkout` | Crear sesión checkout |
| POST | `/api/v1/billing/portal` | Portal de cliente Stripe |
| GET | `/api/v1/billing/subscription` | Suscripción actual |

### Webhooks
| Método | Endpoint | Descripción |
|--------|----------|-------------|
| POST | `/api/v1/webhooks/stripe` | Eventos Stripe (firmados) |

### Health
| Método | Endpoint | Descripción |
|--------|----------|-------------|
| GET | `/api/v1/health` | Health check |

---

## Despliegue en producción

### Opción A: Docker Compose (VPS)
```bash
# En servidor
git clone https://github.com/belentani7/agentguard.git
cd agentguard
cp .env.example .env.production
# Editar .env.production con valores reales
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d
```

### Opción B: Kubernetes (Helm chart pendiente)
```bash
# helm install agentguard ./helm/agentguard
```

### Opción C: Railway / Render / Fly.io
Conecta el repo y usa el Dockerfile incluido. Variables de entorno en dashboard.

### Checklist producción
- [ ] `SECRET_KEY` única y segura (`openssl rand -base64 32`)
- [ ] `STRIPE_WEBHOOK_SECRET` configurado en dashboard Stripe
- [ ] DNS apuntando a IP / LB
- [ ] TLS/SSL (Let's Encrypt / Cloudflare)
- [ ] Backups PostgreSQL automatizados
- [ ] Monitoring: Prometheus + Grafana / Datadog
- [ ] Log aggregation (Loki / Elastic)

---

## Desarrollo local

```bash
# Instalar deps
pip install -r requirements.txt

# Levantar solo DB + Redis
docker compose up -d postgres redis

# Ejecutar API con hot reload
uvicorn app.main:app --reload --port 8000

# Tests
pytest -v

# Lint
ruff check app/
ruff format app/

# Migraciones (Alembic)
alembic revision --autogenerate -m "descripcion"
alembic upgrade head
```

---

## Arquitectura

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Client    │────▶│   API GW    │────▶│  FastAPI    │
│  (Agente)   │     │  (nginx/    │     │  (async)    │
└─────────────┘     │  Cloudflare)│     └──────┬──────┘
                    └─────────────┘              │
                            ┌────────────────────┼────────────────────┐
                            ▼                    ▼                    ▼
                     ┌─────────────┐       ┌─────────────┐       ┌─────────────┐
                     │ PostgreSQL  │       │   Redis     │       │   Stripe    │
                     │  (AsyncPG)  │       │  (Cache/    │       │  (Billing)  │
                     │             │       │   Queue)    │       │             │
                     └─────────────┘       └─────────────┘       └─────────────┘
```

---

## Seguridad

- **gitleaks** en CI — evita secretos en repo
- **Trivy** escaneo vulnerabilidades contenedor
- **Dependabot** actualizaciones automáticas
- **Non-root** container user
- **Read-only** filesystem donde posible
- **CSP** headers en nginx (producción)

---

## Roadmap

- [ ] SDK Python / TypeScript
- [ ] Helm chart Kubernetes
- [ ] Plugin LangChain / LangGraph
- [ ] Plugin CrewAI / AutoGen
- [ ] Dashboard web (React + Tailwind)
- [ ] Marketplace de políticas comunitarias

---

## Licencia

MIT — úsalo, modifícalo, véndelo.  
Construido por **Pedro Belentani** (Trust & Safety Engineer).  
`belentani.eu` · `noiacore.com`
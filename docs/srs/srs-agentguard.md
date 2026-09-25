# SRS -- agentguard
Fecha: 2026-09-25 | Estado: Draft | Traza a: PRD prd-agentguard.md

## Requisitos funcionales

| ID | Requisito | Traza PRD | Prioridad |
|---|---|---|---|
| FR-001 | El sistema implementa: Docker 24+ / Docker Compose 2.20+ | F1 | Must |
| FR-002 | El sistema implementa: Cuenta Stripe (para billing) | F2 | Must |
| FR-003 | El sistema implementa: PostgreSQL 16+ (incluido en compose) | F3 | Must |
| FR-004 | El sistema implementa: [ ] SECRET_KEY única y segura (openssl rand -base64 32) | F4 | Must |
| FR-005 | El sistema implementa: [ ] STRIPE_WEBHOOK_SECRET configurado en dashboard Stripe | F5 | Must |
| FR-006 | El sistema implementa: [ ] DNS apuntando a IP / LB | F6 | Must |
| FR-007 | El sistema implementa: [ ] TLS/SSL (Let's Encrypt / Cloudflare) | F7 | Must |
| FR-008 | El sistema implementa: [ ] Backups PostgreSQL automatizados | F8 | Must |

## Requisitos no funcionales

| ID | Requisito | Metrica | Traza |
|---|---|---|---|
| NFR-001 | Build reproducible | `build` pasa en CI | todos |
| NFR-002 | Calidad estatica | lint + typecheck sin errores | todos |
| NFR-003 | Seguridad | 0 secretos; validacion de entrada | FR-001 |
| NFR-004 | Observabilidad | logs estructurados y errores claros | todos |
| NFR-005 | Accesibilidad (si hay UI) | WCAG 2.1 AA | FR-001 |
| NFR-006 | CI verde | workflow en cada PR | todos |

## Trazabilidad

`PRD -> FR/NFR -> tests -> verificacion`. Todo cambio actualiza la documentacion
en el mismo PR y debe pasar la suite antes de fusionar.

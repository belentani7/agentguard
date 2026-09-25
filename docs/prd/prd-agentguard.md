# PRD -- agentguard
Fecha: 2026-09-25 | Estado: Draft (auditoria automatica, requiere revision humana) | Autor: auditoria belentani7 (NOIACORE)

## 1. Problema

> El escudo que tus agentes necesitan. Cumplimiento, seguridad y observabilidad en una API.

## 2. Usuarios objetivo

- **Primario**: usuario final que necesita resolver el caso de uso de agentguard.
- **Secundario**: equipo/persona que mantiene y despliega el proyecto.
- **Terciario**: agentes CLI que operan sobre el repositorio.

## 3. Features (MoSCoW)

| ID | Feature | MoSCoW |
|---|---|---|
| F1 | Docker 24+ / Docker Compose 2.20+ | Must |
| F2 | Cuenta Stripe (para billing) | Must |
| F3 | PostgreSQL 16+ (incluido en compose) | Must |
| F4 | [ ] SECRET_KEY única y segura (openssl rand -base64 32) | Must |
| F5 | [ ] STRIPE_WEBHOOK_SECRET configurado en dashboard Stripe | Must |
| F6 | [ ] DNS apuntando a IP / LB | Must |
| F7 | [ ] TLS/SSL (Let's Encrypt / Cloudflare) | Must |
| F8 | [ ] Backups PostgreSQL automatizados | Must |
| F90 | Checklist de produccion (build, tests, deploy, seguridad) | Should |
| F91 | Documentacion viva (esta cadena) | Must |

## 4. Criterios de aceptacion (GWT)

### F1 -- Docker 24+ / Docker Compose 2.20+
- Given el usuario en el contexto de agentguard / When usa Docker 24+ / Docker Compose 2.20+ / Then obtiene el resultado esperado sin error.
- Given entrada invalida / When la envia / Then recibe un error generico y el detalle queda en logs.

### F2 -- Cuenta Stripe (para billing)
- Given el usuario en el contexto de agentguard / When usa Cuenta Stripe (para billing) / Then obtiene el resultado esperado sin error.
- Given entrada invalida / When la envia / Then recibe un error generico y el detalle queda en logs.

### F3 -- PostgreSQL 16+ (incluido en compose)
- Given el usuario en el contexto de agentguard / When usa PostgreSQL 16+ (incluido en compose) / Then obtiene el resultado esperado sin error.
- Given entrada invalida / When la envia / Then recibe un error generico y el detalle queda en logs.

### F4 -- [ ] SECRET_KEY única y segura (openssl rand -base64 32)
- Given el usuario en el contexto de agentguard / When usa [ ] SECRET_KEY única y segura (openssl rand -base6 / Then obtiene el resultado esperado sin error.
- Given entrada invalida / When la envia / Then recibe un error generico y el detalle queda en logs.


## 5. Metricas de exito

- Build reproducible en un comando.
- CI verde en cada PR.
- Cero secretos en el repositorio.
- Documentacion actualizada en el mismo PR que el codigo.

## 6. Out of scope

- Funcionalidad no descrita en el README vigente.
- Cambios que rompan compatibilidad sin ADR que lo justifique.

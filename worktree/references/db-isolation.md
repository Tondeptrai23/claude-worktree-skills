# Database Isolation Reference

## Migration tool detection

| Tool | How to detect | Schema-aware? |
|------|--------------|---------------|
| Flyway | `flyway-core` dep, `db/migration/V*.sql` | Yes (`FLYWAY_DEFAULT_SCHEMA`) |
| Liquibase | `liquibase-core` dep, `db.changelog*.xml` | Yes (`defaultSchemaName`) |
| Alembic | `alembic/`, `alembic.ini` | Yes (via `search_path` in URL) |
| Prisma | `prisma/migrations/` | **No** — always uses default schema |
| Django | `*/migrations/`, `manage.py` | **No** — `django_migrations` table conflicts |
| TypeORM | `synchronize: true` in config | **No** — auto-alters on startup |
| Hibernate ddl-auto | `ddl-auto: update` in application.yml | Partial — additive only |

## Isolation modes

### `none` — shared DB, shared schema
No env modifications. Only safe when no migration tool or purely additive changes.

### `schema` — shared DB, per-slot schema (`feature_{N}`)
- Create: `CREATE SCHEMA IF NOT EXISTS feature_{N}`
- Destroy: `DROP SCHEMA IF EXISTS feature_{N} CASCADE`
- JDBC: `?currentSchema=feature_{N}`
- SQLAlchemy: `?options=-c search_path=feature_{N}`
- Flyway: `FLYWAY_DEFAULT_SCHEMA=feature_{N}`
- **Not supported**: Prisma, Django

### `database` — per-slot Docker container
- Create: `docker run -d --name {project}-postgres-slot-{N} -p $((port_base+N)):5432 {image}`
- Wait: poll `pg_isready`
- Seed: run seed script if configured
- Env: point to `localhost:$((port_base + N))`
- Destroy: `docker rm -f {project}-postgres-slot-{N}`

## Recommendation logic

- No migration tool found → `schema` or `none`
- Flyway/Alembic/Liquibase → `schema` works
- Prisma/Django/TypeORM `synchronize` → must use `database`
- Multiple tools across services → use the strictest requirement

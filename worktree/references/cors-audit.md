# CORS Audit Reference

Worktree slots use different origins (`f{N}.localhost`) than the main checkout (`localhost:XXXX`). Restrictive CORS policies will block cross-origin API calls from worktree frontends.

## Where to search per framework

| Framework | Files | Search terms |
|-----------|-------|-------------|
| Spring Boot | `application.yml`, `*Configurer*.java`, `*Filter*.java` | `allowedOrigins`, `ALLOWED_ORIGINS`, `cors`, `@CrossOrigin` |
| Express/Node | `app.js`, `server.js`, middleware | `cors()`, `Access-Control-Allow-Origin` |
| FastAPI | `main.py`, middleware | `CORSMiddleware`, `allow_origins` |
| Django | `settings.py` | `CORS_ALLOWED_ORIGINS`, `django-cors-headers` |
| Go | middleware, `main.go` | `cors`, `AllowOrigins` |
| ASP.NET | `Program.cs`, `Startup.cs` | `AddCors`, `WithOrigins` |

## Classification

| Type | What it looks like | Action |
|------|-------------------|--------|
| **Env-driven** | `ALLOWED_ORIGINS` env var split into list | Add worktree origins to `env_overrides` — include both nginx URLs and `localhost:port` |
| **Hardcoded** | Origins baked into source code | Warn user — must refactor to env var or add `*.localhost` pattern |
| **Pattern-based** | Regex or wildcard like `*.localhost` | Check if it covers worktree subdomains — if yes, no action |
| **Wide-open** | `allowedOrigins("*")` | No action |
| **None configured** | No CORS middleware | Warn — cross-origin will fail |
| **Proxy-based** | Vite `server.proxy` | Warn — nginx bypasses the proxy, need real CORS |

## Related concerns

- **Cookies**: `SameSite`/`Domain` attributes — cookies from `localhost:5173` won't be sent to `f1-api.localhost`
- **OAuth redirect URIs**: Keycloak/Auth0/Google must include worktree origins
- **WebSocket origins**: May validate `Origin` header separately
- **CSP headers**: `connect-src`, `frame-src` may block worktree origins

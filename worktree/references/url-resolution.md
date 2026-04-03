# URL Resolution Reference

When generating `.env.overrides`, resolve `{{service.url}}` templates based on **who consumes the URL** and **whether the service exists in the slot**.

## Server-consumed URLs (backend-to-backend)

Examples: `AI_API_BASE_URL`, `WORKER_API_URL`

- Service in slot: `http://localhost:{port_base + slot * offset}`
- Service NOT in slot: `http://localhost:{port_base}` (main checkout)
- Docker mode: `http://{container_name}:{internal_port}`

## Browser-consumed URLs

Detected by prefix: `VITE_*`, `NEXT_PUBLIC_*`, `REACT_APP_*`, or marked with `# browser` comment.

- Service in slot + nginx: `http://{subdomain}.localhost`
- Service in slot + no nginx: `http://localhost:{port}`
- Service NOT in slot: `http://localhost:{port_base}`
- Docker mode: still use localhost/nginx (browser is on host)

## Port formula

`port = port_base + (slot * port_offset)`

Default `port_offset` is 100. Slot 0 is the main checkout.

| Slot | +100 offset example |
|------|-------------------|
| 0 | 8080, 3000, 8081 |
| 1 | 8180, 3100, 8181 |
| 2 | 8280, 3200, 8281 |
| 3 | 8380, 3300, 8381 |

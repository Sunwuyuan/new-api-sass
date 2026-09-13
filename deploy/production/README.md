# Production deploy (image only)

Production hosts **must not clone the git repo**. Only pull the published image.

Image: `ghcr.io/sunwuyuan/new-api-sass` (built by `.github/workflows/saas-package.yml`).

On the server keep only:

- `docker-compose.yml`
- `Caddyfile`
- `certs/cert.pem` + `certs/key.pem` (self-signed or real TLS; generate on host)
- `.env` (secrets; never commit)
- `data/` `logs/` volumes

```bash
cd /opt/new-api-sass
docker compose pull
docker compose up -d
```

Required `.env`: `IMAGE`, `PLATFORM_ORIGIN` (HTTPS), `PLATFORM_ADMIN_EMAIL`, `PLATFORM_ADMIN_PASSWORD`,
`SESSION_SECRET`, `CRYPTO_SECRET`, `DB_PASSWORD`, `REDIS_PASSWORD`, `SESSION_COOKIE_SECURE=true`, `SESSION_COOKIE_TRUSTED_URL` (exact HTTPS origin, same as PLATFORM_ORIGIN).

For IP-only HTTPS without a domain, generate a self-signed cert on the host:

```bash
mkdir -p certs
openssl req -x509 -newkey rsa:2048 -nodes -days 825 \
  -keyout certs/key.pem -out certs/cert.pem \
  -subj "/CN=YOUR_IP" -addext "subjectAltName=IP:YOUR_IP"
```

Browsers will warn until you attach a real domain + Let's Encrypt.

## Auto-update (Watchtower)

`watchtower` polls every 5 minutes and recreates only the `new-api` container when
`ghcr.io/sunwuyuan/new-api-sass:latest` changes. Postgres / Redis / Caddy are not auto-updated.

Disable: `docker compose stop watchtower` (or remove the service).


# Production deploy (Docker Compose)

Image: `ghcr.io/sunwuyuan/new-api-sass` (built by `.github/workflows/saas-package.yml`).

On the server:

```bash
sudo mkdir -p /opt/new-api-sass && sudo chown "$USER:$USER" /opt/new-api-sass
cd /opt/new-api-sass
# place docker-compose.yml + .env (never commit .env)
docker compose pull
docker compose up -d
```

Required `.env` keys: `PLATFORM_ORIGIN`, `PLATFORM_ADMIN_EMAIL`, `PLATFORM_ADMIN_PASSWORD`,
`SESSION_SECRET`, `CRYPTO_SECRET`, `DB_PASSWORD`, `REDIS_PASSWORD`.

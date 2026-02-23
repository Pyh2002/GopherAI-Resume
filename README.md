# GopherAI-Resume

Step 1 bootstrap for backend engineering baseline.

## Features in this stage
- Gin HTTP server with graceful shutdown.
- Config loading from `configs/config.toml` + environment variable overrides.
- MySQL / Redis / RabbitMQ connection bootstrap.
- `/healthz` endpoint for dependency health checks.
- Auth API: register/login/me with bcrypt + JWT.
- Chat baseline API: session create/list + message send/history (non-streaming).
- Simple web pages: `/`, `/login`, `/register`, `/chat`.
- Redis cache-aside for chat history with dirty-marker protection.

## Quick start
1. Ensure MySQL, Redis, RabbitMQ are running in WSL.
2. Copy `.env.example` values into your shell (or set equivalent env vars).
3. Create database:
   - `mysql -uroot -e "CREATE DATABASE IF NOT EXISTS gopherai_resume CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"`
4. Run:
   - `make tidy`
   - `make run`
5. Verify:
   - `curl http://127.0.0.1:8080/healthz`

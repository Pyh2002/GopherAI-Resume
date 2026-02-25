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

## Image recognition (optional)

The vision feature uses ONNX Runtime to run the MobileNetV2 model. The Go binding requires the **native ONNX Runtime shared library** on your machine (separate from the model file in `assets/`).

### Quick setup (Linux / WSL)

From the project root:

```bash
make install-onnx-runtime
```

Then set the printed `VISION_ONNX_LIB` in your environment and run the app (e.g. `export VISION_ONNX_LIB=...` then `make run`), or set `configs/config.toml` under `[vision]` → `onnx_shared_lib_path`.

### Manual setup (Linux)

1. **Download** the CPU build for your arch, e.g. x64:
   - https://github.com/microsoft/onnxruntime/releases/download/v1.24.1/onnxruntime-linux-x64-1.24.1.tgz
   - For ARM64: `onnxruntime-linux-aarch64-1.24.1.tgz`
2. **Extract** and point the app at the `.so`:
   ```bash
   mkdir -p bin/onnxruntime
   tar -xzf onnxruntime-linux-x64-1.24.1.tgz -C bin/onnxruntime
   export VISION_ONNX_LIB="$(pwd)/bin/onnxruntime/onnxruntime-linux-x64-1.24.1/lib/libonnxruntime.so.1.24.1"
   ```
   (Adjust the path if the tarball layout differs; use `find bin/onnxruntime -name 'libonnxruntime.so*'` to locate the file.)
3. **Run** the app with the same env, or set `onnx_shared_lib_path` in `configs/config.toml` under `[vision]`.

Model and labels are read from `assets/` by default: `assets/mobilenetv2-7.onnx` and `assets/labels.txt`.

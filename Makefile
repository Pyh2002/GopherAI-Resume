APP_NAME := gopherai-resume
ONNX_VERSION := 1.24.1
ONNX_ARCH := linux-x64
ONNX_LIB := bin/onnxruntime/onnxruntime-$(ONNX_ARCH)-$(ONNX_VERSION)/lib/libonnxruntime.so.$(ONNX_VERSION)

.PHONY: tidy build run install-onnx-runtime ensure-onnx

tidy:
	go mod tidy

build:
	go build -o bin/$(APP_NAME) ./cmd/server

# Ensure ONNX Runtime .so exists; download and extract if missing.
ensure-onnx: $(ONNX_LIB)

$(ONNX_LIB):
	@mkdir -p bin/onnxruntime
	@echo "Downloading ONNX Runtime $(ONNX_VERSION) ($(ONNX_ARCH))..."
	@curl -sSL -o bin/onnxruntime/onnxruntime.tgz \
	  "https://github.com/microsoft/onnxruntime/releases/download/v$(ONNX_VERSION)/onnxruntime-$(ONNX_ARCH)-$(ONNX_VERSION).tgz"
	@tar -xzf bin/onnxruntime/onnxruntime.tgz -C bin/onnxruntime --no-same-owner
	@echo "ONNX Runtime ready at $(ONNX_LIB)"

# Build ONNX lib if needed, then run the Go server with VISION_ONNX_LIB set.
run: ensure-onnx
	VISION_ONNX_LIB="$(CURDIR)/$(ONNX_LIB)" go run ./cmd/server

# Download ONNX Runtime shared library for Linux (required for image recognition).
# Optional: "make run" now does this automatically. Use this target to install only.
install-onnx-runtime: $(ONNX_LIB)
	@LIB="$(CURDIR)/$(ONNX_LIB)"; \
	echo ""; echo "Set the library path and run:"; \
	echo "  export VISION_ONNX_LIB=\"$$LIB\""; \
	echo "  make run"

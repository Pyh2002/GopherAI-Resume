package handler

import (
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"gopherai-resume/internal/vision"
	"gopherai-resume/internal/transport/http/response"
)

const maxImageSize = 5 << 20 // 5 MB

// VisionHandler handles image classification requests.
type VisionHandler struct {
	classifier *vision.Classifier
}

// NewVisionHandler creates a vision handler that uses the given classifier.
func NewVisionHandler(classifier *vision.Classifier) *VisionHandler {
	return &VisionHandler{classifier: classifier}
}

// ClassifyRequest can optionally send top_k in JSON body; we use form "image" for the file.
// TopK is otherwise from config (default 5).

// Classify accepts a multipart form with "image" (image file), runs local MobileNetV2 classification, returns top-k labels.
func (h *VisionHandler) Classify(c *gin.Context) {
	file, err := c.FormFile("image")
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "missing image file (form field 'image')")
		return
	}

	if file.Size > maxImageSize {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "image too large (max 5MB)")
		return
	}

	f, err := file.Open()
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "failed to open uploaded file")
		return
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "failed to read image")
		return
	}

	results, err := h.classifier.Classify(data)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "cannot open shared object file") || strings.Contains(msg, "Error loading ONNX shared library") {
			msg = "ONNX Runtime library not found. Install it and set VISION_ONNX_LIB to the path to libonnxruntime.so (see README)."
		} else {
			msg = "classification failed: " + msg
		}
		response.Error(c, http.StatusServiceUnavailable, response.CodeInternalServer, msg)
		return
	}

	response.OK(c, gin.H{"predictions": results})
}

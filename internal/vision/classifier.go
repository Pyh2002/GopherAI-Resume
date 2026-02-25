package vision

import (
	"bufio"
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"sort"
	"strings"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
	"golang.org/x/image/draw"
)

// ImageNet normalization (standard for torchvision models).
var (
	imagenetMean = [3]float32{0.485, 0.456, 0.406}
	imagenetStd  = [3]float32{0.229, 0.224, 0.225}
)

const (
	width  = 224
	height = 224
)

// LabelScore holds a class label and its score (logit or probability).
type LabelScore struct {
	Label string  `json:"label"`
	Index int     `json:"index"`
	Score float32 `json:"score"`
}

// Classifier runs MobileNetV2 ONNX inference and maps outputs to labels.
type Classifier struct {
	mu sync.Mutex

	modelPath  string
	labelsPath string
	topK       int
	libPath    string

	session *ort.AdvancedSession
	input   *ort.Tensor[float32]
	output  *ort.Tensor[float32]
	labels  []string
	inited  bool
}

// NewClassifier creates a classifier that will lazily load the ONNX model and labels.
func NewClassifier(modelPath, labelsPath, onnxLibPath string, topK int) *Classifier {
	if topK <= 0 {
		topK = 5
	}
	return &Classifier{
		modelPath:  modelPath,
		labelsPath: labelsPath,
		topK:       topK,
		libPath:    onnxLibPath,
	}
}

// initOnce loads the ONNX shared library, environment, labels, and session.
func (c *Classifier) initOnce() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.inited {
		return nil
	}

	if c.libPath != "" {
		ort.SetSharedLibraryPath(c.libPath)
	}

	if err := ort.InitializeEnvironment(); err != nil {
		return fmt.Errorf("onnx init environment: %w", err)
	}

	labels, err := loadLabels(c.labelsPath)
	if err != nil {
		return fmt.Errorf("load labels: %w", err)
	}
	c.labels = labels

	inputs, outputs, err := ort.GetInputOutputInfo(c.modelPath)
	if err != nil {
		return fmt.Errorf("onnx get input/output info: %w", err)
	}
	if len(inputs) == 0 || len(outputs) == 0 {
		return fmt.Errorf("onnx model has no inputs or outputs")
	}
	inputShape := inputs[0].Dimensions
	outputShape := outputs[0].Dimensions

	inputTensor, err := ort.NewEmptyTensor[float32](inputShape)
	if err != nil {
		return fmt.Errorf("onnx new input tensor: %w", err)
	}
	c.input = inputTensor

	outputTensor, err := ort.NewEmptyTensor[float32](outputShape)
	if err != nil {
		inputTensor.Destroy()
		return fmt.Errorf("onnx new output tensor: %w", err)
	}
	c.output = outputTensor

	inputNames := make([]string, len(inputs))
	for i := range inputs {
		inputNames[i] = inputs[i].Name
	}
	outputNames := make([]string, len(outputs))
	for i := range outputs {
		outputNames[i] = outputs[i].Name
	}

	session, err := ort.NewAdvancedSession(c.modelPath, inputNames, outputNames,
		[]ort.Value{c.input}, []ort.Value{c.output}, nil)
	if err != nil {
		outputTensor.Destroy()
		inputTensor.Destroy()
		return fmt.Errorf("onnx new session: %w", err)
	}
	c.session = session
	c.inited = true
	return nil
}

func loadLabels(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var labels []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		labels = append(labels, strings.TrimSpace(sc.Text()))
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return labels, nil
}

// Classify decodes the image, preprocesses it for MobileNetV2, runs inference, and returns top-k label scores.
func (c *Classifier) Classify(imageData []byte) ([]LabelScore, error) {
	if err := c.initOnce(); err != nil {
		return nil, err
	}

	img, err := decodeImage(imageData)
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	// Preprocess: resize to 224x224, RGB, NCHW, ImageNet normalized float32.
	inputData := preprocess(img)
	if len(inputData) == 0 {
		return nil, fmt.Errorf("preprocess failed")
	}

	c.mu.Lock()
	inData := c.input.GetData()
	if len(inData) < len(inputData) {
		c.mu.Unlock()
		return nil, fmt.Errorf("input tensor size %d < preprocessed %d", len(inData), len(inputData))
	}
	copy(inData, inputData)
	err = c.session.Run()
	c.mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("onnx run: %w", err)
	}

	outData := c.output.GetData()
	k := c.topK
	if k > len(c.labels) {
		k = len(c.labels)
	}
	if k > len(outData) {
		k = len(outData)
	}

	// Top-k by score (logits).
	type idxScore struct {
		idx   int
		score float32
	}
	scored := make([]idxScore, len(outData))
	for i, s := range outData {
		scored[i] = idxScore{i, s}
	}
	sort.Slice(scored, func(i, j int) bool { return scored[i].score > scored[j].score })

	result := make([]LabelScore, 0, k)
	for i := 0; i < k; i++ {
		idx := scored[i].idx
		label := ""
		if idx < len(c.labels) {
			label = c.labels[idx]
		}
		result = append(result, LabelScore{
			Label: label,
			Index: idx,
			Score: scored[i].score,
		})
	}
	return result, nil
}

func decodeImage(data []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		// Try JPEG and PNG explicitly (image.Decode may not recognize some)
		img, err = jpeg.Decode(bytes.NewReader(data))
		if err != nil {
			img, err = png.Decode(bytes.NewReader(data))
			if err != nil {
				return nil, err
			}
		}
	}
	return img, nil
}

// preprocess resizes img to 224x224, converts to RGB, NCHW layout, float32 with ImageNet normalization.
func preprocess(img image.Image) []float32 {
	bounds := img.Bounds()

	// Draw into 224x224 RGBA using bilinear scaling.
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, bounds, draw.Over, nil)

	// NCHW: [1, 3, 224, 224] -> 1*3*224*224 floats.
	out := make([]float32, 1*3*height*width)
	const size = width * height

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := y*width + x
			c := dst.RGBAAt(x, y)
			r, g, b := float32(c.R)/255.0, float32(c.G)/255.0, float32(c.B)/255.0
			out[0*size+idx] = (r - imagenetMean[0]) / imagenetStd[0]
			out[1*size+idx] = (g - imagenetMean[1]) / imagenetStd[1]
			out[2*size+idx] = (b - imagenetMean[2]) / imagenetStd[2]
		}
	}
	return out
}

// DecodeImageFromReader decodes an image from r (e.g. multipart form file). Used by handler.
func DecodeImageFromReader(r io.Reader) (image.Image, error) {
	img, _, err := image.Decode(r)
	if err != nil {
		return nil, err
	}
	return img, nil
}

// PreprocessImage converts an image.Image to the float32 NCHW tensor slice for MobileNetV2.
func PreprocessImage(img image.Image) []float32 {
	return preprocess(img)
}

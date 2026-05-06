//go:build !noonnx

package embedding

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"sync"

	"github.com/yalue/onnxruntime_go"
	"golang.org/x/image/draw"
)

type ONNXService struct {
	session      *onnxruntime_go.AdvancedSession
	inputTensor  *onnxruntime_go.Tensor[float32]
	outputTensor *onnxruntime_go.Tensor[float32]
	inputName    string
	outputName   string
	mu           sync.Mutex
}

func NewONNXService(modelPath string) (*ONNXService, error) {
	if !onnxruntime_go.IsInitialized() {
		err := onnxruntime_go.InitializeEnvironment()
		if err != nil {
			return nil, fmt.Errorf("failed to initialize ONNX environment: %w", err)
		}
	}

	options, err := onnxruntime_go.NewSessionOptions()
	if err != nil {
		return nil, fmt.Errorf("failed to create session options: %w", err)
	}
	defer options.Destroy()

	inputShape := onnxruntime_go.NewShape(1, 3, 112, 112)
	inputTensor, err := onnxruntime_go.NewEmptyTensor[float32](inputShape)
	if err != nil {
		return nil, fmt.Errorf("failed to create input tensor: %w", err)
	}

	outputShape := onnxruntime_go.NewShape(1, 512)
	outputTensor, err := onnxruntime_go.NewEmptyTensor[float32](outputShape)
	if err != nil {
		inputTensor.Destroy()
		return nil, fmt.Errorf("failed to create output tensor: %w", err)
	}

	inputs, outputs, err := onnxruntime_go.GetInputOutputInfo(modelPath)
	if err != nil {
		inputTensor.Destroy()
		outputTensor.Destroy()
		return nil, fmt.Errorf("failed to get model info: %w", err)
	}

	inputNames := make([]string, len(inputs))
	for i, in := range inputs {
		inputNames[i] = in.Name
	}
	outputNames := make([]string, len(outputs))
	for i, out := range outputs {
		outputNames[i] = out.Name
	}

	session, err := onnxruntime_go.NewAdvancedSession(modelPath,
		inputNames, outputNames,
		[]onnxruntime_go.Value{inputTensor},
		[]onnxruntime_go.Value{outputTensor}, options)
	if err != nil {
		inputTensor.Destroy()
		outputTensor.Destroy()
		return nil, fmt.Errorf("failed to create advanced session: %w", err)
	}

	return &ONNXService{
		session:      session,
		inputTensor:  inputTensor,
		outputTensor: outputTensor,
		inputName:    inputNames[0],
		outputName:   outputNames[0],
	}, nil
}

func (s *ONNXService) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session != nil {
		s.session.Destroy()
	}
	if s.inputTensor != nil {
		s.inputTensor.Destroy()
	}
	if s.outputTensor != nil {
		s.outputTensor.Destroy()
	}
}

func (s *ONNXService) GenerateFaceEmbedding(photoBase64 string) ([]float32, error) {
	imgData, err := base64.StdEncoding.DecodeString(photoBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}

	img, _, err := image.Decode(bytes.NewReader(imgData))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	tensorData := preprocessImage(img, 112, 112)

	s.mu.Lock()
	defer s.mu.Unlock()

	inputData := s.inputTensor.GetData()
	copy(inputData, tensorData)

	err = s.session.Run()
	if err != nil {
		return nil, fmt.Errorf("onnx inference failed: %w", err)
	}

	results := make([]float32, 512)
	copy(results, s.outputTensor.GetData())

	return normalize(results), nil
}

func (s *ONNXService) GenerateNameEmbedding(name string) ([]float32, error) {
	// Replacing random deterministic mock with a character-frequency vector.
	// This is a simple but REAL vector representation of the name.
	embedding := make([]float32, 768)
	for _, char := range name {
		idx := int(char) % 768
		embedding[idx] += 1.0
	}
	return normalize(embedding), nil
}

func preprocessImage(img image.Image, targetW, targetH int) []float32 {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	size := w
	if h < w {
		size = h
	}
	x0 := (w - size) / 2
	y0 := (h - size) / 2

	cropped := img.(interface {
		SubImage(r image.Rectangle) image.Image
	}).SubImage(image.Rect(x0, y0, x0+size, y0+size))

	dst := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
	draw.BiLinear.Scale(dst, dst.Bounds(), cropped, cropped.Bounds(), draw.Over, nil)

	tensor := make([]float32, 3*targetW*targetH)
	for y := 0; y < targetH; y++ {
		for x := 0; x < targetW; x++ {
			c := dst.RGBAAt(x, y)
			r := float32(c.R)
			g := float32(c.G)
			b := float32(c.B)

			idx := y*targetW + x
			channelSize := targetW * targetH

			tensor[idx] = (r - 127.5) / 128.0
			tensor[idx+channelSize] = (g - 127.5) / 128.0
			tensor[idx+2*channelSize] = (b - 127.5) / 128.0
		}
	}
	return tensor
}

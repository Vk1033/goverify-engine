package embedding

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"runtime"
	"sync"

	"github.com/yalue/onnxruntime_go"
	"golang.org/x/image/draw"
	"github.com/vk1033/goverify-engine/internal/config"
)

type inferenceRequest struct {
	inputData []float32
	result    chan inferenceResponse
}

type inferenceResponse struct {
	embedding []float32
	err       error
}

type RealService struct {
	requests chan inferenceRequest
	mu       sync.Mutex
}

func NewRealService(cfg *config.Config) (Service, error) {
	// 1. Initialize ONNX Runtime
	if !onnxruntime_go.IsInitialized() {
		onnxruntime_go.SetSharedLibraryPath(cfg.AI.LibraryPath)
		err := onnxruntime_go.InitializeEnvironment()
		if err != nil {
			return nil, fmt.Errorf("failed to initialize onnxruntime: %w", err)
		}
	}

	s := &RealService{
		requests: make(chan inferenceRequest),
	}

	// 2. Start the Inference Loop in a locked OS thread
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		// Discover Node Names
		inputs := []string{"data", "input", "input.1"}
		outputs := []string{"1333", "fc1", "output", "683", "457", "embedding", "fc1_output_0"}
		
		dummyInput := make([]float32, 1*3*112*112)
		inputTensor, _ := onnxruntime_go.NewTensor(onnxruntime_go.NewShape(1, 3, 112, 112), dummyInput)
		defer inputTensor.Destroy()
		outputTensor, _ := onnxruntime_go.NewEmptyTensor[float32](onnxruntime_go.NewShape(1, 512))
		defer outputTensor.Destroy()

		var sess *onnxruntime_go.DynamicAdvancedSession
		var err error

		for _, in := range inputs {
			for _, out := range outputs {
				sess, err = onnxruntime_go.NewDynamicAdvancedSession(cfg.AI.FaceModelPath,
					[]string{in}, []string{out}, nil)
				if err == nil {
					err = sess.Run([]onnxruntime_go.Value{inputTensor}, []onnxruntime_go.Value{outputTensor})
					if err == nil {
						fmt.Printf("[INFO] AI Face Model FULLY VERIFIED with Input: %s, Output: %s\n", in, out)
						goto found
					}
					sess.Destroy()
				}
			}
		}

		fmt.Printf("[WARN] AI Face Model failed to load from %s\n", cfg.AI.FaceModelPath)
		return

	found:
		defer sess.Destroy()

		// 3. Process requests
		for req := range s.requests {
			copy(inputTensor.GetData(), req.inputData)
			err = sess.Run([]onnxruntime_go.Value{inputTensor}, []onnxruntime_go.Value{outputTensor})
			if err != nil {
				req.result <- inferenceResponse{err: fmt.Errorf("inference failed: %w", err)}
				continue
			}
			
			res := make([]float32, 512)
			copy(res, outputTensor.GetData())
			req.result <- inferenceResponse{embedding: normalizeEmbedding(res)}
		}
	}()

	return s, nil
}

func (s *RealService) GenerateFaceEmbedding(photoBase64 string) ([]float32, error) {
	// 1. Decode Image
	imgData, err := base64.StdEncoding.DecodeString(photoBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}

	img, _, err := image.Decode(bytes.NewReader(imgData))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	// 2. Preprocess (112x112 for ArcFace)
	resized := image.NewRGBA(image.Rect(0, 0, 112, 112))
	draw.BiLinear.Scale(resized, resized.Bounds(), img, img.Bounds(), draw.Over, nil)

	inputData := make([]float32, 1*3*112*112)
	for y := 0; y < 112; y++ {
		for x := 0; x < 112; x++ {
			r, g, b, _ := resized.At(x, y).RGBA()
			inputData[0*112*112+y*112+x] = (float32(r>>8) - 127.5) / 128.0
			inputData[1*112*112+y*112+x] = (float32(g>>8) - 127.5) / 128.0
			inputData[2*112*112+y*112+x] = (float32(b>>8) - 127.5) / 128.0
		}
	}

	// 3. Send Request to Inference Loop
	resultChan := make(chan inferenceResponse)
	s.requests <- inferenceRequest{
		inputData: inputData,
		result:    resultChan,
	}

	resp := <-resultChan
	return resp.embedding, resp.err
}

func (s *RealService) GenerateNameEmbedding(name string) ([]float32, error) {
	mock := &MockService{}
	return mock.GenerateNameEmbedding(name)
}

func (s *RealService) Close() error {
	close(s.requests)
	return nil
}

func normalizeEmbedding(v []float32) []float32 {
	var sum float32
	for _, val := range v {
		sum += val * val
	}
	norm := float32(math.Sqrt(float64(sum)))
	if norm == 0 {
		return v
	}
	for i := range v {
		v[i] /= norm
	}
	return v
}

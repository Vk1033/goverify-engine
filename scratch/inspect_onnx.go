
package main

import (
	"fmt"
	"github.com/yalue/onnxruntime_go"
)

func main() {
	onnxruntime_go.SetSharedLibraryPath("/usr/local/lib/libonnxruntime.so")
	err := onnxruntime_go.InitializeEnvironment()
	if err != nil {
		panic(err)
	}
	defer onnxruntime_go.DestroyEnvironment()

	inspect("models/facenet512.onnx")
	inspect("models/retinaface.onnx")
}

func inspect(path string) {
	fmt.Printf("=== %s ===\n", path)
	session, err := onnxruntime_go.NewAdvancedSession(path, nil, nil, nil)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer session.Destroy()

	fmt.Println("Inputs:")
	for _, input := range session.GetInputNames() {
		fmt.Println(" ", input)
	}
	fmt.Println("Outputs:")
	for _, output := range session.GetOutputNames() {
		fmt.Println(" ", output)
	}
	fmt.Println()
}

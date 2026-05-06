
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

	session, err := onnxruntime_go.NewAdvancedSession("models/face.onnx", nil, nil, nil)
	if err != nil {
		panic(err)
	}
	defer session.Destroy()

	fmt.Println("Inputs:")
	for _, input := range session.GetInputNames() {
		fmt.Println(input)
	}
	fmt.Println("Outputs:")
	for _, output := range session.GetOutputNames() {
		fmt.Println(output)
	}
}

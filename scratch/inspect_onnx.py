
import onnx
model = onnx.load("models/face.onnx")
print("Inputs:")
for input in model.graph.input:
    print(input.name)
print("\nOutputs:")
for output in model.graph.output:
    print(output.name)

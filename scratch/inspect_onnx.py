import onnx

def inspect(model_path):
    print(f"=== Inspecting {model_path} ===")
    model = onnx.load(model_path)
    print("Inputs:")
    for input in model.graph.input:
        shape = [dim.dim_value if dim.HasField("dim_value") else dim.dim_param for dim in input.type.tensor_type.shape.dim]
        print(f"  {input.name}: {shape}")
    print("\nOutputs:")
    for output in model.graph.output:
        shape = [dim.dim_value if dim.HasField("dim_value") else dim.dim_param for dim in output.type.tensor_type.shape.dim]
        print(f"  {output.name}: {shape}")
    print()

inspect("models/facenet512.onnx")
inspect("models/retinaface.onnx")

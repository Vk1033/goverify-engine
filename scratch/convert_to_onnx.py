import os
import tensorflow as tf
import tf2onnx
from deepface import DeepFace

def convert_facenet512():
    print("Loading FaceNet512 model from DeepFace...")
    model = DeepFace.build_model("Facenet512")
    
    print("Converting FaceNet512 to ONNX...")
    spec = (tf.TensorSpec((None, 160, 160, 3), tf.float32, name="input"),)
    output_path = "models/facenet512.onnx"
    tf2onnx.convert.from_keras(model, input_signature=spec, opt_level=13, output_path=output_path)
    print(f"Saved to {output_path}")

if __name__ == "__main__":
    os.makedirs("models", exist_ok=True)
    convert_facenet512()

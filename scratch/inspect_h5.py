import urllib.request
import h5py

print("Downloading facenet512...")
urllib.request.urlretrieve("https://github.com/serengil/deepface_models/releases/download/v1.0/facenet512_weights.h5", "facenet512_weights.h5")

print("Downloading retinaface...")
urllib.request.urlretrieve("https://github.com/serengil/deepface_models/releases/download/v1.0/retinaface.h5", "retinaface.h5")

def inspect(file):
    try:
        with h5py.File(file, 'r') as f:
            if 'model_config' in f.attrs:
                print(f"{file}: Contains model architecture (full model)")
            else:
                print(f"{file}: Contains only weights")
    except Exception as e:
        print(f"{file}: Error reading - {e}")

inspect("facenet512_weights.h5")
inspect("retinaface.h5")

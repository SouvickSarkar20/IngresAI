import requests
import os
from dotenv import load_dotenv

load_dotenv()

API_KEY = os.getenv("GEMINI_API_KEY")
if not API_KEY:
    raise ValueError("GEMINI_API_KEY not found in environment variables")
    
models_to_test = [
    "gemini-flash-latest",
    "gemini-flash-lite-latest",
    "gemini-3.5-flash",
    "gemini-3.6-flash",
    "gemini-3.7-flash"
]

for model in models_to_test:
    url = f"https://generativelanguage.googleapis.com/v1beta/models/{model}:generateContent?key={API_KEY}"
    resp = requests.post(url, json={'contents':[{'parts':[{'text':'Hello'}]}]})
    print(f"{model}: {resp.status_code}")
    if resp.status_code != 200:
        print(f"  Error: {resp.json().get('error', {}).get('message', 'Unknown')}")

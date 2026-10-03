import os
import hashlib
from fastapi import FastAPI, Request, Response
import httpx

app = FastAPI()

RECORD_MODE = os.getenv("RECORD_MODE", "true").lower() == "true"
STUBS_DIR = "stubs"

os.makedirs(STUBS_DIR, exist_ok=True)

def get_hash(body: bytes) -> str:
    return hashlib.md5(body).hexdigest()

@app.post("/api/v1/chat/completions")
async def proxy_openrouter(request: Request):
    body = await request.body()
    req_hash = get_hash(body)
    stub_path = os.path.join(STUBS_DIR, f"openrouter_{req_hash}.json")
    
    if not RECORD_MODE and os.path.exists(stub_path):
        print(f"[REPLAY] OpenRouter hit -> {req_hash}")
        with open(stub_path, "rb") as f:
            return Response(content=f.read(), media_type="application/json")
            
    print(f"[{'RECORD' if RECORD_MODE else 'MISS'}] Proxying OpenRouter -> {req_hash}")
    headers = dict(request.headers)
    headers.pop("host", None)
    
    async with httpx.AsyncClient() as client:
        resp = await client.post(
            "https://openrouter.ai/api/v1/chat/completions",
            content=body,
            headers=headers,
            timeout=120.0
        )
        
    if RECORD_MODE:
        with open(stub_path, "wb") as f:
            f.write(resp.content)
            
    return Response(content=resp.content, status_code=resp.status_code, headers=dict(resp.headers))

@app.post("/api/gec/getBusinessDataForUserOpen")
async def proxy_ingres(request: Request):
    body = await request.body()
    req_hash = get_hash(body)
    stub_path = os.path.join(STUBS_DIR, f"ingres_{req_hash}.json")
    
    if not RECORD_MODE and os.path.exists(stub_path):
        print(f"[REPLAY] INGRES hit -> {req_hash}")
        with open(stub_path, "rb") as f:
            return Response(content=f.read(), media_type="application/json")
            
    print(f"[{'RECORD' if RECORD_MODE else 'MISS'}] Proxying INGRES -> {req_hash}")
    headers = dict(request.headers)
    headers.pop("host", None)
    
    async with httpx.AsyncClient() as client:
        resp = await client.post(
            "https://ingres.iith.ac.in/api/gec/getBusinessDataForUserOpen",
            content=body,
            headers=headers,
            timeout=120.0
        )
        
    if RECORD_MODE:
        with open(stub_path, "wb") as f:
            f.write(resp.content)
            
    return Response(content=resp.content, status_code=resp.status_code, headers=dict(resp.headers))

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8080)

import os
import json
import hashlib
import argparse
import urllib.request
import urllib.error
from http.server import BaseHTTPRequestHandler, HTTPServer

TARGET_URL = ""
MODE = "record"
MAPPINGS_DIR = "mappings"

class ProxyHTTPRequestHandler(BaseHTTPRequestHandler):
    def get_request_hash(self, body_bytes):
        """Create a unique hash based on method, path, and request body."""
        signature = f"{self.command}|{self.path}|{body_bytes.decode('utf-8', errors='ignore')}"
        return hashlib.md5(signature.encode('utf-8')).hexdigest()

    def do_ANY(self):
        content_length = int(self.headers.get('Content-Length', 0))
        req_body = self.rfile.read(content_length) if content_length > 0 else b""
        
        req_hash = self.get_request_hash(req_body)
        mapping_file = os.path.join(MAPPINGS_DIR, f"{req_hash}.json")
        
        # --- PLAYBACK MODE ---
        if MODE == "playback":
            found_stub_path = None
            for root, _, files in os.walk(MAPPINGS_DIR):
                if f"{req_hash}.json" in files:
                    found_stub_path = os.path.join(root, f"{req_hash}.json")
                    break
            
            if found_stub_path:
                with open(found_stub_path, 'r', encoding='utf-8') as f:
                    stub = json.load(f)
                
                resp_data = stub.get('response', {})
                self.send_response(resp_data.get('status', 200))
                for k, v in resp_data.get('headers', {}).items():
                    self.send_header(k, v)
                self.end_headers()
                
                body = resp_data.get('body', '')
                self.wfile.write(body.encode('utf-8'))
                print(f"[PLAYBACK] Replayed {self.command} {self.path} (Hash: {req_hash})")
                return
            else:
                try:
                    os.makedirs('tests', exist_ok=True)
                    with open('tests/failed_request.json', 'w') as err_f:
                        err_f.write(req_body.decode('utf-8'))
                except Exception as e:
                    print(e)
                self.send_error(404, f"Stub not found for {self.command} {self.path} (Hash: {req_hash})")
                return
                
        # --- RECORD MODE ---
        target_endpoint = f"{TARGET_URL}{self.path}"
        print(f"[RECORD] Forwarding {self.command} {target_endpoint}...")
        
        req_headers = dict(self.headers)
        # Remove Host so urllib uses the target's host instead of localhost
        if 'Host' in req_headers:
            del req_headers['Host']
        # Remove Accept-Encoding so the target sends raw JSON, not gzipped binary
        if 'Accept-Encoding' in req_headers:
            del req_headers['Accept-Encoding']
            
        req = urllib.request.Request(
            url=target_endpoint,
            data=req_body if req_body else None,
            headers=req_headers,
            method=self.command
        )
        
        import time
        max_retries = 10
        for attempt in range(max_retries):
            try:
                with urllib.request.urlopen(req) as response:
                    resp_status = response.status
                    resp_headers = dict(response.headers)
                    resp_body = response.read()
                    break
            except urllib.error.HTTPError as e:
                resp_status = e.code
                resp_headers = dict(e.headers)
                resp_body = e.read()
                if resp_status in [429, 503]:
                    print(f"  -> Got {resp_status}, waiting 15s before retry ({attempt+1}/{max_retries})...")
                    time.sleep(15)
                    continue
                else:
                    break
            
        # Write response back to the client
        self.send_response(resp_status)
        for k, v in resp_headers.items():
            if k.lower() not in ['transfer-encoding', 'content-encoding', 'connection', 'content-length']:
                self.send_header(k, v)
        self.send_header('Content-Length', str(len(resp_body)))
        self.end_headers()
        self.wfile.write(resp_body)
        
        # Redact API keys from the saved file
        safe_headers = dict(self.headers)
        if 'Authorization' in safe_headers:
            safe_headers['Authorization'] = 'Bearer [REDACTED]'
            
        import re
        safe_path = re.sub(r'key=[^&]+', 'key=REDACTED', self.path)

        # Save Stub as JSON
        stub = {
            "request": {
                "method": self.command,
                "path": safe_path,
                "headers": safe_headers,
                "body": req_body.decode('utf-8', errors='ignore')
            },
            "response": {
                "status": resp_status,
                "headers": resp_headers,
                "body": resp_body.decode('utf-8', errors='ignore')
            }
        }
        
        with open(mapping_file, 'w', encoding='utf-8') as f:
            json.dump(stub, f, indent=2)
            
        print(f"  -> Saved stub: {req_hash}.json")

    # Map all standard HTTP verbs to the handler
    def do_GET(self): self.do_ANY()
    def do_POST(self): self.do_ANY()
    def do_PUT(self): self.do_ANY()
    def do_DELETE(self): self.do_ANY()
    def do_PATCH(self): self.do_ANY()

if __name__ == '__main__':
    parser = argparse.ArgumentParser(description="Lightweight Zero-Dependency Recording Proxy")
    parser.add_argument("--port", type=int, default=8080)
    parser.add_argument("--target", required=True, help="Target URL (e.g., https://openrouter.ai)")
    parser.add_argument("--mode", choices=["record", "playback"], default="record")
    parser.add_argument("--dir", default="mappings", help="Directory to save/load stubs")
    args = parser.parse_args()
    
    TARGET_URL = args.target.rstrip('/')
    MODE = args.mode
    MAPPINGS_DIR = args.dir
    
    os.makedirs(MAPPINGS_DIR, exist_ok=True)
    
    print(f"[*] Starting Zero-Dependency Proxy on port {args.port}")
    print(f"[*] Target: {TARGET_URL}")
    print(f"[*] Mode: {MODE.upper()}")
    print(f"[*] Stubs Directory: {MAPPINGS_DIR}")
    
    server = HTTPServer(('0.0.0.0', args.port), ProxyHTTPRequestHandler)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\nShutting down proxy...")
        server.server_close()

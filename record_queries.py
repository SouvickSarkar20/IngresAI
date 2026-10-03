import csv
import requests
import time
import os

CSV_PATH = r"C:\Users\souvick\.gemini\antigravity-ide\brain\562ae3cf-2c62-4475-b643-76bc083945dd\golden_queries.csv"
AGENT_URL = "http://localhost:9000/agent/chat"

def main():
    if not os.path.exists(CSV_PATH):
        print(f"CSV file not found at {CSV_PATH}")
        return

    with open(CSV_PATH, 'r', encoding='utf-8') as f:
        reader = csv.DictReader(f)
        queries = list(reader)

    print(f"Found {len(queries)} queries. Starting record run...")

    for i, row in enumerate(queries):
        query_id = row['query_id']
        question = row['query_text']
        
        print(f"[{i+1}/{len(queries)}] Sending {query_id}: {question}")
        
        payload = {
            "userId": "system_recorder",
            "chatId": query_id,
            "question": question
        }
        
        try:
            resp = requests.post(AGENT_URL, json=payload, timeout=60)
            resp.raise_for_status()
            print(f"  -> Success: {resp.status_code}")
        except Exception as e:
            print(f"  -> Failed: {e}")
        
        # small delay to prevent rate-limits from upstream APIs during recording
        time.sleep(2)
        
    print("Record run complete! Stubs should be saved in wiremock-llm/mappings and wiremock-ingres/mappings.")

if __name__ == "__main__":
    main()

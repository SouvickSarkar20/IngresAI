import csv
import requests
import time
import os
import json

# The golden queries CSV lives in the artifact dir from when it was created
CSV_PATH = r"C:\Users\souvick\.gemini\antigravity-ide\brain\562ae3cf-2c62-4475-b643-76bc083945dd\golden_queries.csv"
AGENT_URL = "http://localhost:9000/agent/chat"

# Checkpoint file: tracks which query_ids have been successfully recorded
CHECKPOINT_FILE = "recording_checkpoint.json"

def load_checkpoint():
    if os.path.exists(CHECKPOINT_FILE):
        with open(CHECKPOINT_FILE, 'r') as f:
            return set(json.load(f))
    return set()

def save_checkpoint(completed_ids):
    with open(CHECKPOINT_FILE, 'w') as f:
        json.dump(list(completed_ids), f, indent=2)

def main():
    if not os.path.exists(CSV_PATH):
        print(f"CSV file not found at {CSV_PATH}")
        return

    with open(CSV_PATH, 'r', encoding='utf-8') as f:
        reader = csv.DictReader(f)
        queries = list(reader)

    completed = load_checkpoint()
    remaining = [q for q in queries if q['query_id'] not in completed]

    print(f"Total queries: {len(queries)}")
    print(f"Already recorded (skipping): {len(completed)}")
    print(f"Remaining to record: {len(remaining)}")

    if not remaining:
        print("All queries already recorded! Delete recording_checkpoint.json to start fresh.")
        return

    for i, row in enumerate(remaining):
        query_id = row['query_id']
        question = row['query_text']
        category = row['category']

        print(f"\n[{i+1}/{len(remaining)}] {query_id} ({category})")
        print(f"  Q: {question}")

        payload = {
            "userId": "system_recorder",
            "chatId": query_id,
            "question": question
        }

        # Retry up to 5 times at the recorder level too
        success = False
        for attempt in range(5):
            try:
                resp = requests.post(AGENT_URL, json=payload, timeout=120)
                resp.raise_for_status()
                print(f"  -> Success (HTTP {resp.status_code})")
                completed.add(query_id)
                save_checkpoint(completed)
                success = True
                break
            except requests.exceptions.HTTPError as e:
                print(f"  -> HTTP Error {e.response.status_code} on attempt {attempt+1}/5, retrying in 20s...")
                time.sleep(20)
            except Exception as e:
                print(f"  -> Failed: {e} on attempt {attempt+1}/5, retrying in 20s...")
                time.sleep(20)

        if not success:
            print(f"  !! Giving up on {query_id} after 5 attempts. Will retry on next run.")

        # Delay between queries to avoid bursting the LLM API.
        # The proxy already retries on 429/503 from Gemini itself,
        # but this outer delay spaces out total query load.
        if i < len(remaining) - 1:
            print(f"  ... waiting 2s before next query ...")
            time.sleep(2)

    completed_final = load_checkpoint()
    print(f"\n=== Recording session complete ===")
    print(f"Total recorded: {len(completed_final)}/{len(queries)}")
    if len(completed_final) < len(queries):
        missed = [q['query_id'] for q in queries if q['query_id'] not in completed_final]
        print(f"Still missing: {missed}")
        print(f"Re-run this script to pick up where you left off.")
    else:
        print("All 24 queries successfully recorded!")

if __name__ == "__main__":
    main()

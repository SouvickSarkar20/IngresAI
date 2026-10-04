import os
import json
import csv
import shutil
import glob
import re

CSV_PATH = r"C:\Users\souvick\.gemini\antigravity-ide\brain\562ae3cf-2c62-4475-b643-76bc083945dd\golden_queries.csv"
MAPPINGS_LLM = "mappings/llm"
MAPPINGS_INGRES = "mappings/ingres"
FIXTURES_DIR = "tests/fixtures"

def sanitize_dir_name(name):
    # Remove any invalid characters for Windows paths
    return re.sub(r'[<>:"/\\|?*]', '_', name).strip()

def main():
    if not os.path.exists(FIXTURES_DIR):
        os.makedirs(FIXTURES_DIR)
        
    # 1. Read queries
    queries = []
    with open(CSV_PATH, 'r', encoding='utf-8') as f:
        reader = csv.DictReader(f)
        for row in reader:
            queries.append(row)
            
    print(f"Loaded {len(queries)} golden queries.")
    
    # 2. Process LLM Stubs
    llm_files = glob.glob(os.path.join(MAPPINGS_LLM, "*.json"))
    matched_llm_files = set()
    
    for row in queries:
        q_id = row['query_id']
        q_cat = sanitize_dir_name(row['category'])
        q_text = row['query_text']
        
        target_dir = os.path.join(FIXTURES_DIR, q_cat, q_id, "llm")
        os.makedirs(target_dir, exist_ok=True)
        
        # Find all stubs that belong to this query
        for llm_file in llm_files:
            with open(llm_file, 'r', encoding='utf-8') as f:
                content = f.read()
                
            # If the query text is in the request body (history), it belongs to this query
            if q_text in content:
                filename = os.path.basename(llm_file)
                dest = os.path.join(target_dir, filename)
                shutil.copy2(llm_file, dest)
                matched_llm_files.add(llm_file)
                
    print(f"Organized {len(matched_llm_files)}/{len(llm_files)} LLM stubs into specific query folders.")
    
    # Check for unmatched LLM files
    unmatched = set(llm_files) - matched_llm_files
    if unmatched:
        print(f"Warning: {len(unmatched)} LLM stubs did not match any query text. Copying to shared/llm_unmatched.")
        unmatched_dir = os.path.join(FIXTURES_DIR, "shared", "llm_unmatched")
        os.makedirs(unmatched_dir, exist_ok=True)
        for f in unmatched:
            shutil.copy2(f, os.path.join(unmatched_dir, os.path.basename(f)))
            
    # 3. Process Ingres (Gov API) Stubs
    # Since Gov API requests are identical global requests fetching all map data, 
    # we put them in a shared fixture folder.
    ingres_files = glob.glob(os.path.join(MAPPINGS_INGRES, "*.json"))
    if ingres_files:
        shared_ingres_dir = os.path.join(FIXTURES_DIR, "shared", "ingres")
        os.makedirs(shared_ingres_dir, exist_ok=True)
        for f in ingres_files:
            shutil.copy2(f, os.path.join(shared_ingres_dir, os.path.basename(f)))
        print(f"Copied {len(ingres_files)} Gov API stubs to shared/ingres.")
        
    print(f"Fixtures successfully frozen in {FIXTURES_DIR}!")

if __name__ == "__main__":
    main()

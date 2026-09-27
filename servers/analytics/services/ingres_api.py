import os
import requests

INGRES_API_URL = os.environ.get("INGRES_API_URL", "https://ingres.iith.ac.in/api/gec/getBusinessDataForUserOpen")

def fetch_raw_data(location: str, year: str, view: str, locuuid: str) -> dict:
    """
    Fetches raw data from the INGRES API and filters by location name.
    The API always returns all states for the INDIA-level query.
    We then search for the matching locationName (case-insensitive).
    """
    payload = {
        "parentLocName": "INDIA",
        "locname": "INDIA",
        "loctype": "COUNTRY",
        "view": view,
        "locuuid": "ffce954d-24e1-494b-ba7e-0931d8ad6085",
        "year": year,
        "computationType": "normal",
        "component": "recharge",
        "period": "annual",
        "category": "safe",
        "mapOnClickParams": "false",
        "verificationStatus": 1,
        "approvalLevel": 1,
        "parentuuid": "ffce954d-24e1-494b-ba7e-0931d8ad6085",
        "stateuuid": None
    }
    
    response = requests.post(INGRES_API_URL, json=payload)
    response.raise_for_status()
    data = response.json()
    
    # Search for the matching location by name (case-insensitive)
    search_loc = location.strip().lower()
    
    if data and isinstance(data, list):
        for item in data:
            loc_name = item.get("locationName", "")
            if loc_name.strip().lower() == search_loc:
                print(f"✅ Found match: {loc_name} (SOE: {item.get('stageOfExtraction', {}).get('total', 'N/A')})")
                return {"records": len(data), "sample": item}
        
        print(f"⚠️ No match for '{location}'. Available: {[i.get('locationName') for i in data[:5]]}")
    
    return {"records": 0, "sample": {}}
def fetch_locations() -> list:
    """
    Fetches the full list of locations from the INGRES API.
    """
    payload = {
        "parentLocName": "INDIA",
        "locname": "INDIA",
        "loctype": "COUNTRY",
        "view": "admin",
        "locuuid": "ffce954d-24e1-494b-ba7e-0931d8ad6085",
        "year": "2023-2024",
        "computationType": "normal",
        "component": "recharge",
        "period": "annual",
        "category": "safe",
        "mapOnClickParams": "false",
        "verificationStatus": 1,
        "approvalLevel": 1,
        "parentuuid": "ffce954d-24e1-494b-ba7e-0931d8ad6085",
        "stateuuid": None
    }
    
    response = requests.post(INGRES_API_URL, json=payload)
    response.raise_for_status()
    data = response.json()
    
    locations = []
    seen_names = set()
    
    if data and isinstance(data, list):
        for item in data:
            name = item.get("locationName")
            uuid = item.get("locationUUID")
            if name and name not in seen_names:
                locations.append({"label": name, "value": name, "uuid": uuid})
                seen_names.add(name)
                
    # Sort alphabetically for better UX
    locations.sort(key=lambda x: x["label"])
    return locations

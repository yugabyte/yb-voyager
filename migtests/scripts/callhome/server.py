import json
import logging
import os
import requests

from flask import Flask, request, jsonify

application = Flask(__name__)
# Store migration phase data
callhome_payload = {}

# Helper for standardized responses
def api_response(status_code, message):
    if status_code != 200:
        resp = jsonify({"error": message})
        logging.error(message)
    else:
        resp = jsonify(message)
    resp.status_code = status_code
    return resp

def clean_payload(payload):
    # If the payload is a string, check if it contains JSON
    if isinstance(payload, str):
        try:
            # Attempt to parse the string as JSON
            parsed = json.loads(payload)
            return clean_payload(parsed)
        except json.JSONDecodeError:
            return payload
    # If the payload is a dictionary, clean each key-value pair
    elif isinstance(payload, dict):
        return {k: clean_payload(v) for k, v in payload.items()}
    else:
        return payload

# Endpoint to retrieve stored payload for migration phase
@application.route("/get_payload/<phase>", methods=["GET"])
def get_payload(phase):
    if phase not in callhome_payload:
        return api_response(404, "Phase not found")
    return api_response(200, callhome_payload[phase]);

# Endpoint for receiving callhome data
@application.route("/", methods=["POST"])
def diagnostics():
    global callhome_payload
    try:
        data = json.loads(request.get_data()) 
        error = False

        try:
            # Parse nested payload
            payload = clean_payload(data)
            if data['migration_phase'] in callhome_payload and data['migration_phase'] == "import-schema":
                # Avoid overwriting existing import-schema payload as finalize-schema command has the same migration phase as import-schema
                data['migration_phase'] = "finalize-schema"

            # Store payload by phase
            callhome_payload[data['migration_phase']] = payload

        except Exception as e:
            logging.error(f"Error processing payload: {e}")
            error = True

        if error:
            return api_response(400, "Error processing payload")
        return api_response(200, "Success")

    except Exception as e:
        logging.error(f"Error in diagnostics: {e}")
        return api_response(500, str(e))
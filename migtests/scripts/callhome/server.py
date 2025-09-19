import json
import logging
import os
import requests

from flask import Flask, request, jsonify

application = Flask(__name__)

def api_response(status_code, message):
    if status_code != 200:
        resp = jsonify({"error": message})
        logging.error(message)
    else:
        resp = jsonify(message)
    resp.status_code = status_code
    return resp


@application.route("/", methods=["POST"])
def diagnostics():
    try:
        data = json.loads(request.get_data())
        error = False
        ip_address = request.headers.get("X-Forwarded-For", request.remote_addr)
        data["host_ip"] = ip_address

        try:
            # Sort arrays to ensure consistent ordering
            payload = json.loads(data['phase_payload'])
            test_dir=os.environ.get("TEST_DIR")
            with open(f"{test_dir}/actualCallhomeReport.json", "w") as f:
                json.dump(payload, f, indent=4)
        except Exception as e:
            logging.error(f"Error processing payload: {e}")
            error = True

        if error:
            return api_response(400, "Error processing payload")
        return api_response(200, {"success": "true"})

    except Exception as e:
        logging.error(f"Error in diagnostics: {e}")
        return api_response(500, str(e))
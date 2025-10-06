import json
import logging
import os
import requests

from flask import Flask, request, jsonify

application = Flask(__name__)
callhome_payload = {}

def api_response(status_code, message):
    if status_code != 200:
        resp = jsonify({"error": message})
        logging.error(message)
    else:
        resp = jsonify(message)
    resp.status_code = status_code
    return resp

@application.route("/get_report", methods=["GET"])
def get_report():
    return api_response(200, callhome_payload);

@application.route("/", methods=["POST"])
def diagnostics():
    global callhome_payload
    try:
        data = json.loads(request.get_data())
        error = False
        ip_address = request.headers.get("X-Forwarded-For", request.remote_addr)
        data["host_ip"] = ip_address

        try:
            payload = json.loads(data['phase_payload'])
            callhome_payload = payload
        except Exception as e:
            logging.error(f"Error processing payload: {e}")
            error = True

        if error:
            return api_response(400, "Error processing payload")
        return api_response(200, {"success": "true"})

    except Exception as e:
        logging.error(f"Error in diagnostics: {e}")
        return api_response(500, str(e))
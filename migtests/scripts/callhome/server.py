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
            payload = data['phase_payload']
            
            # If payload is a string, parse it as JSON first
            if isinstance(payload, str):
                payload = json.loads(payload)
            
            # Sort colocated_tables array if it exists
            if 'sizing' in payload and 'colocated_tables' in payload['sizing']:
                payload['sizing']['colocated_tables'].sort()
            
            # Sort assessment_issues array if it exists
            if 'assessment_issues' in payload:
                payload['assessment_issues'].sort(key=lambda x: str(x))
            
            # Sort anonymized_ddls array if it exists
            if 'anonymized_ddls' in payload:
                payload['anonymized_ddls'].sort()

            if 'yb_cluster_metrics' in payload:
                payload['yb_cluster_metrics']='{}'
            
            # Sort table_sizing_stats and index_sizing_stats (JSON strings containing arrays)
            for field in ['table_sizing_stats', 'index_sizing_stats']:
                if field in payload and isinstance(payload[field], str):
                    try:
                        parsed_data = json.loads(payload[field])
                        if isinstance(parsed_data, list):
                            parsed_data.sort(key=lambda x: str(x))
                            payload[field] = json.dumps(parsed_data)
                    except json.JSONDecodeError:
                        # If it's not valid JSON, leave it as is
                        pass

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
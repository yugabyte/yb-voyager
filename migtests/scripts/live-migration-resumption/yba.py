#!/usr/bin/env python3
"""YugabyteDB Anywhere (YBA) helper for the upgrade / rolling-restart live-migration tests.

Two things live here:

1. `YBAClient`: the small REST client the orchestrator actions `yba_cluster_event`,
   `yba_wait_task` and `yba_check_version` use to drive the target universe mid-migration.
   It covers only what the tests need: read the universe record, register a release,
   run the software-upgrade precheck, start a rolling software upgrade / finalize /
   rolling restart, and poll the resulting task. It is written against the YBA REST API
   (`/universes/{uuid}/upgrade/*`, `/tasks/{uuid}`, `/ybdb_release`), the same endpoints
   perf-devops' perfstudio uses for its Upgrade Universe step. Only GET and POST are
   allowed on purpose: the token is shared with the perf team and must never delete.

   Connection details come from the environment only (YBA_URL, YBA_API_TOKEN,
   YBA_UNIVERSE_UUID, optional YBA_CA_FILE): the rendered scenario file is archived by
   Jenkins, so the token must never be written into it.

2. `yba.py resolve-versions`: picks the pair of builds the pipeline upgrades between and
   prints `OLD_VERSION=...` / `NEW_VERSION=...` for Jenkins to export before it creates
   the universe. NEW is the latest released build of the newest stable branch and OLD the
   latest released build of the previous one, both from release.dev.yugabyte.com. Nothing
   else is consulted: the upgrade stage registers NEW on the YBA itself if it is missing,
   and YBA's own precheck rejects a pair it cannot upgrade.
"""

import argparse
import json
import os
import re
import ssl
import sys
import time
import urllib.error
import urllib.request
from typing import Any, Dict, List, Optional

RELEASE_SERVICE_URL = "https://release.dev.yugabyte.com"
RELEASE_PACKAGE_URL = "https://s3.us-west-2.amazonaws.com/releases.yugabyte.com/{build}/yugabyte-{build}-centos-x86_64.tar.gz"
HTTP_TIMEOUT_SEC = 60
GET_ATTEMPTS = 3
GET_RETRY_SLEEP_SEC = 5

YB_BUILD_RE = re.compile(r"^(\d+)\.(\d+)\.(\d+)\.(\d+)-b(\d+)$")
STABLE_BRANCH_RE = re.compile(r"^\d{4}\.\d+$")
# Release tarballs report -b0 from `select version()`, so only the version number is usable.
YB_SERVER_VERSION_RE = re.compile(r"-YB-(\d+\.\d+\.\d+\.\d+)-b\d+")

TASK_STATUS_SUCCESS = "Success"
TASK_STATUS_TERMINAL_FAILURE = {"Failure", "Aborted", "Abort", "Unknown"}

UPGRADE_STATE_PRE_FINALIZE = "PreFinalize"


def version_key(build: str) -> tuple:
    m = YB_BUILD_RE.match(build)
    if not m:
        raise ValueError(f"not a YugabyteDB build string: {build!r}")
    return tuple(int(x) for x in m.groups())


def version_number(build: str) -> str:
    version_key(build)
    return build.split("-b", 1)[0]


def parse_server_version_number(version_output: str) -> str:
    m = YB_SERVER_VERSION_RE.search(version_output)
    if not m:
        raise ValueError(f"no -YB-<version>-b<build> token in: {version_output!r}")
    return m.group(1)


def _branch_key(branch: str) -> tuple:
    return tuple(int(x) for x in branch.split("."))


def _http_get_text(url: str) -> str:
    with urllib.request.urlopen(url, timeout=HTTP_TIMEOUT_SEC) as resp:
        return resp.read().decode("utf-8", "replace")


def stable_branches() -> List[str]:
    entries = json.loads(_http_get_text(f"{RELEASE_SERVICE_URL}/version/active/json"))
    branches = sorted(
        {e["branch"] for e in entries if STABLE_BRANCH_RE.match(str(e.get("branch", "")))},
        key=_branch_key,
        reverse=True,
    )
    if not branches:
        raise RuntimeError(f"no YYYY.N branches in {RELEASE_SERVICE_URL}/version/active/json: {entries}")
    return branches


def released_build(branch: str) -> Optional[str]:
    answer = _http_get_text(f"{RELEASE_SERVICE_URL}/version/released/{branch}").strip()
    if answer == "None" or not answer:
        return None
    version_key(answer)
    return answer


class YBAClient:
    def __init__(self, base_url: str, api_token: str, universe_uuid: Optional[str] = None):
        self.base_url = base_url.rstrip("/")
        self._api_token = api_token
        self.universe_uuid = universe_uuid
        self._customer_uuid: Optional[str] = None
        # The perf-team YBA presents a self-signed certificate; without YBA_CA_FILE the
        # connection is unverified on the internal network only.
        ca_file = os.environ.get("YBA_CA_FILE")
        self._ssl = ssl.create_default_context(cafile=ca_file)
        if not ca_file:
            self._ssl.check_hostname = False
            self._ssl.verify_mode = ssl.CERT_NONE

    @classmethod
    def from_env(cls) -> "YBAClient":
        required = ["YBA_URL", "YBA_API_TOKEN", "YBA_UNIVERSE_UUID"]
        missing = [k for k in required if not os.environ.get(k)]
        if missing:
            raise RuntimeError(f"missing environment variables: {', '.join(missing)}")
        return cls(os.environ["YBA_URL"], os.environ["YBA_API_TOKEN"], os.environ["YBA_UNIVERSE_UUID"])

    def _request(self, method: str, path: str, body: Any = None) -> Any:
        if method not in ("GET", "POST"):
            raise ValueError(f"{method} is not allowed: the shared YBA token must never delete anything")
        url = f"{self.base_url}/api/v1{path}"
        data = json.dumps(body).encode() if body is not None else None
        req = urllib.request.Request(
            url,
            data=data,
            method=method,
            headers={
                "X-AUTH-YW-API-TOKEN": self._api_token,
                "Content-Type": "application/json",
                "Accept": "application/json",
            },
        )
        # A POST is never retried: a repeated upgrade/restart would submit the task twice.
        attempts = GET_ATTEMPTS if method == "GET" else 1
        for attempt in range(1, attempts + 1):
            try:
                with urllib.request.urlopen(req, timeout=HTTP_TIMEOUT_SEC, context=self._ssl) as resp:
                    raw = resp.read()
                return json.loads(raw) if raw else {}
            except urllib.error.HTTPError as exc:
                detail = exc.read().decode("utf-8", "replace")[:2000]
                error = RuntimeError(f"YBA {method} {path}: HTTP {exc.code}: {detail}")
                retryable = exc.code >= 500
            except (urllib.error.URLError, TimeoutError, OSError) as exc:
                error = RuntimeError(f"YBA {method} {path}: {exc}")
                retryable = True
            if not retryable or attempt == attempts:
                raise error from None
            time.sleep(GET_RETRY_SLEEP_SEC)
        raise AssertionError("unreachable")

    @property
    def customer_uuid(self) -> str:
        if self._customer_uuid is None:
            self._customer_uuid = self._request("GET", "/session_info")["customerUUID"]
        return self._customer_uuid

    def _universe_path(self, suffix: str = "") -> str:
        return f"/customers/{self.customer_uuid}/universes/{self.universe_uuid}{suffix}"

    def universe(self) -> Dict[str, Any]:
        return self._request("GET", self._universe_path())

    @staticmethod
    def _primary_cluster(universe: Dict[str, Any]) -> Dict[str, Any]:
        clusters = universe["universeDetails"]["clusters"]
        primaries = [c for c in clusters if c.get("clusterType") == "PRIMARY"]
        if len(primaries) != 1:
            raise RuntimeError(f"expected exactly one PRIMARY cluster, found {len(primaries)}")
        return primaries[0]

    @staticmethod
    def universe_version(universe: Dict[str, Any]) -> str:
        return YBAClient._primary_cluster(universe)["userIntent"]["ybSoftwareVersion"]

    @staticmethod
    def software_upgrade_state(universe: Dict[str, Any]) -> str:
        details = universe["universeDetails"]
        if "softwareUpgradeState" not in details:
            raise RuntimeError("universe details carry no softwareUpgradeState; this YBA cannot report finalize state")
        return details["softwareUpgradeState"]

    def register_release(self, build: str) -> None:
        """Make `build` installable on this YBA, the way perf-devops' PortalOps does: one POST
        with the releases.yugabyte.com tarball URL; an already registered build is fine."""
        version_key(build)
        body = {
            "version": build,
            "release_tag": "",
            "yb_type": "YBDB",
            "artifacts": [{"platform": "LINUX", "architecture": "x86_64", "package_url": RELEASE_PACKAGE_URL.format(build=build)}],
            "release_type": "PREVIEW",
        }
        try:
            self._request("POST", f"/customers/{self.customer_uuid}/ybdb_release", body)
        except RuntimeError as exc:
            if "already exists" not in str(exc):
                raise

    def _rolling_payload(self, universe: Dict[str, Any]) -> Dict[str, Any]:
        details = universe["universeDetails"]
        if details.get("updateInProgress"):
            raise RuntimeError(
                f"universe {self.universe_uuid} has another task in progress "
                f"(updateInProgress=true, updatingTask={details.get('updatingTask')})"
            )
        return {
            "universeUUID": self.universe_uuid,
            "nodePrefix": details["nodePrefix"],
            "clusters": details["clusters"],
            "upgradeOption": "Rolling",
        }

    def software_upgrade_precheck(self, version: str) -> Dict[str, Any]:
        version_key(version)
        return self._request("POST", self._universe_path("/upgrade/software/precheck"), {"ybSoftwareVersion": version})

    def start_upgrade(self, version: str) -> str:
        version_key(version)
        universe = self.universe()
        payload = self._rolling_payload(universe)
        payload.update(
            {
                "taskType": "Software",
                "ybSoftwareVersion": version,
                "enableYbc": universe["universeDetails"].get("enableYbc", True),
            }
        )
        return self._request("POST", self._universe_path("/upgrade/db_version"), payload)["taskUUID"]

    def start_restart(self) -> str:
        payload = self._rolling_payload(self.universe())
        payload["ybSoftwareVersion"] = None
        return self._request("POST", self._universe_path("/upgrade/restart"), payload)["taskUUID"]

    def start_finalize(self) -> str:
        return self._request("POST", self._universe_path("/upgrade/finalize"), {})["taskUUID"]

    def task(self, task_uuid: str) -> Dict[str, Any]:
        return self._request("GET", f"/customers/{self.customer_uuid}/tasks/{task_uuid}")

    def failed_subtasks(self, task_uuid: str) -> List[Dict[str, Any]]:
        resp = self._request("GET", f"/customers/{self.customer_uuid}/tasks/{task_uuid}/failed")
        return resp.get("failedSubTasks", [])


def resolve_versions(report=print) -> Dict[str, str]:
    branches = stable_branches()
    report(f"stable branches from {RELEASE_SERVICE_URL}/version/active/json: {branches}")
    released: List[str] = []
    for branch in branches:
        build = released_build(branch)
        report(f"  /version/released/{branch} -> {build}")
        if build:
            released.append(build)
    released.sort(key=version_key, reverse=True)
    if len(released) < 2:
        raise RuntimeError(f"need released builds on two branches, found {released}")
    return {"OLD_VERSION": released[1], "NEW_VERSION": released[0]}


def main(argv: Optional[List[str]] = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = parser.add_subparsers(dest="command", required=True)
    sub.add_parser("resolve-versions", help="print the OLD_VERSION/NEW_VERSION pair for the upgrade tests")
    args = parser.parse_args(argv)

    if args.command == "resolve-versions":
        versions = resolve_versions(report=lambda m: print(m, file=sys.stderr))
        for name, value in versions.items():
            print(f"{name}={value}")
        return 0
    return 2


if __name__ == "__main__":
    sys.exit(main())

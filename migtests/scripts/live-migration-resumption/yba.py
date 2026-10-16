#!/usr/bin/env python3
"""YugabyteDB Anywhere client used by the live-migration resumption orchestrator.

Connection details come from the environment only (YBA_URL, YBA_API_TOKEN,
YBA_UNIVERSE_UUID, optional YBA_CA_FILE): the rendered scenario file is archived
by Jenkins, so the token must never be written into it.

    yba.py resolve-versions [--latest] [--skip-yba-check]

prints OLD_VERSION=... and NEW_VERSION=... for the pipeline to export before it
creates the universe.
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
HTTP_TIMEOUT_SEC = 60
GET_ATTEMPTS = 3
GET_RETRY_SLEEP_SEC = 5

YB_BUILD_RE = re.compile(r"^(\d+)\.(\d+)\.(\d+)\.(\d+)-b(\d+)$")
STABLE_BRANCH_RE = re.compile(r"^\d{4}\.\d+$")
# Release tarballs report -b0 from `select version()`, so only the version number is usable.
YB_SERVER_VERSION_RE = re.compile(r"-YB-(\d+\.\d+\.\d+\.\d+)-b\d+")

# The released pair latest-2025.2 -> latest-2026.1 is rejected by YBA's CheckUpgrade until
# 2026.1.3.0 ships (2025.2.7.0 carries YSQL migration 30566, 2026.1.2.0 does not), so the
# nightly runs a pinned supported pair; `--latest` switches back to the release-service pick.
PINNED_OLD_VERSION = "2025.2.6.0-b111"
PINNED_NEW_VERSION = "2026.1.2.0-b137"

TASK_STATUS_SUCCESS = "Success"
TASK_STATUS_TERMINAL_FAILURE = {"Failure", "Aborted", "Abort", "Unknown"}

UPGRADE_STATE_READY = "Ready"
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
    def from_env(cls, require_universe: bool = True) -> "YBAClient":
        required = ["YBA_URL", "YBA_API_TOKEN"] + (["YBA_UNIVERSE_UUID"] if require_universe else [])
        missing = [k for k in required if not os.environ.get(k)]
        if missing:
            raise RuntimeError(f"missing environment variables: {', '.join(missing)}")
        return cls(os.environ["YBA_URL"], os.environ["YBA_API_TOKEN"], os.environ.get("YBA_UNIVERSE_UUID"))

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
        if not self.universe_uuid:
            raise RuntimeError("YBA_UNIVERSE_UUID is not set")
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

    @staticmethod
    def tserver_private_ips(universe: Dict[str, Any]) -> List[str]:
        return [
            n["cloudInfo"]["private_ip"]
            for n in universe["universeDetails"]["nodeDetailsSet"]
            if n.get("isTserver")
        ]

    def release_versions(self) -> List[str]:
        return [r["version"] for r in self._request("GET", f"/customers/{self.customer_uuid}/ybdb_release")]

    def _rolling_payload(self, universe: Dict[str, Any], sleep_between_servers_sec: int) -> Dict[str, Any]:
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
            "sleepAfterMasterRestartMillis": sleep_between_servers_sec * 1000,
            "sleepAfterTServerRestartMillis": sleep_between_servers_sec * 1000,
        }

    def software_upgrade_precheck(self, version: str) -> Dict[str, Any]:
        version_key(version)
        return self._request("POST", self._universe_path("/upgrade/software/precheck"), {"ybSoftwareVersion": version})

    def start_upgrade(self, version: str, sleep_between_servers_sec: int) -> str:
        version_key(version)
        universe = self.universe()
        payload = self._rolling_payload(universe, sleep_between_servers_sec)
        payload.update(
            {
                "taskType": "Software",
                "ybSoftwareVersion": version,
                "enableYbc": universe["universeDetails"].get("enableYbc", True),
            }
        )
        return self._request("POST", self._universe_path("/upgrade/db_version"), payload)["taskUUID"]

    def start_restart(self, sleep_between_servers_sec: int) -> str:
        payload = self._rolling_payload(self.universe(), sleep_between_servers_sec)
        payload["ybSoftwareVersion"] = None
        return self._request("POST", self._universe_path("/upgrade/restart"), payload)["taskUUID"]

    def start_finalize(self) -> str:
        return self._request("POST", self._universe_path("/upgrade/finalize"), {})["taskUUID"]

    def task(self, task_uuid: str) -> Dict[str, Any]:
        return self._request("GET", f"/customers/{self.customer_uuid}/tasks/{task_uuid}")

    def failed_subtasks(self, task_uuid: str) -> List[Dict[str, Any]]:
        resp = self._request("GET", f"/customers/{self.customer_uuid}/tasks/{task_uuid}/failed")
        return resp.get("failedSubTasks", [])


def _build_on_yba(released: str, on_yba: List[str], report) -> str:
    """Return the released build when the YBA has it, else the highest YBA build of the same version.

    The perf YBA often carries only in-development builds of a released version, so the
    version is what the test holds fixed and the build is reported, not required.
    """
    if released in on_yba:
        return released
    number = version_number(released)
    same_version = sorted(
        (b for b in on_yba if YB_BUILD_RE.match(b) and version_number(b) == number),
        key=version_key,
    )
    if not same_version:
        raise RuntimeError(f"no build of {number} is imported into the YBA (released build is {released})")
    report(f"WARNING: released build {released} is not on the YBA; using {same_version[-1]} (released: {released})")
    return same_version[-1]


def pinned_versions(yba: Optional[YBAClient], report=print) -> Dict[str, str]:
    report(f"pinned pair: OLD={PINNED_OLD_VERSION} NEW={PINNED_NEW_VERSION}")
    if yba is not None:
        on_yba = set(yba.release_versions())
        missing = [v for v in (PINNED_OLD_VERSION, PINNED_NEW_VERSION) if v not in on_yba]
        if missing:
            raise RuntimeError(f"pinned builds not imported into YBA {yba.base_url}: {missing}")
        report(f"both builds present on {yba.base_url}")
    return {
        "OLD_VERSION": PINNED_OLD_VERSION,
        "NEW_VERSION": PINNED_NEW_VERSION,
        "OLD_VERSION_RELEASED": PINNED_OLD_VERSION,
        "NEW_VERSION_RELEASED": PINNED_NEW_VERSION,
    }


def resolve_versions(yba: Optional[YBAClient], report=print) -> Dict[str, str]:
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
    new_released, old_released = released[0], released[1]
    new, old = new_released, old_released
    if yba is not None:
        on_yba = yba.release_versions()
        new = _build_on_yba(new_released, on_yba, report)
        old = _build_on_yba(old_released, on_yba, report)
        report(f"builds to use from {yba.base_url}: OLD={old} NEW={new}")
    return {
        "OLD_VERSION": old,
        "NEW_VERSION": new,
        "OLD_VERSION_RELEASED": old_released,
        "NEW_VERSION_RELEASED": new_released,
    }


def main(argv: Optional[List[str]] = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = parser.add_subparsers(dest="command", required=True)
    resolve = sub.add_parser("resolve-versions", help="print the OLD_VERSION/NEW_VERSION pair for the upgrade tests")
    resolve.add_argument(
        "--latest",
        action="store_true",
        help="pick latest released builds from the release service instead of the pinned pair",
    )
    resolve.add_argument(
        "--skip-yba-check",
        action="store_true",
        help="do not confirm the builds exist on the YBA (no YBA_* env needed)",
    )
    args = parser.parse_args(argv)

    if args.command == "resolve-versions":
        yba = None if args.skip_yba_check else YBAClient.from_env(require_universe=False)
        pick = resolve_versions if args.latest else pinned_versions
        versions = pick(yba, report=lambda m: print(m, file=sys.stderr))
        for name, value in versions.items():
            print(f"{name}={value}")
        return 0
    return 2


if __name__ == "__main__":
    sys.exit(main())

#!/usr/bin/env bash
# Build the voyager-sandbox image. Run this from anywhere inside the repo.
#
# Usage:
#   bash docker/sandbox/build.sh            # builds voyager-sandbox:latest
#   IMAGE=voyager-sandbox:dev bash ...      # override tag
#   YB_VERSION=2025.2.2.2 YB_BUILD=b11 ...  # override YugabyteDB version

set -euo pipefail

REPO="$(git rev-parse --show-toplevel)"
IMAGE="${IMAGE:-voyager-sandbox:latest}"
YB_VERSION="${YB_VERSION:-2025.2.2.2}"
YB_BUILD="${YB_BUILD:-b11}"
GO_VERSION="${GO_VERSION:-1.24.2}"

cd "$REPO"

# By default we build for the host architecture (native) -- much faster than
# qemu emulation, especially for the Maven/Go compile steps. Override with
# PLATFORM=linux/amd64 if you specifically need an x86_64 image.
echo "Building ${IMAGE} (YB=${YB_VERSION}-${YB_BUILD}, Go=${GO_VERSION}, platform=${PLATFORM:-host-native})"
echo "This takes ~10-15 minutes on first run; subsequent builds reuse layer cache."

PLATFORM_ARG=()
if [ -n "${PLATFORM:-}" ]; then
    PLATFORM_ARG=(--platform="${PLATFORM}")
fi

docker build \
    "${PLATFORM_ARG[@]}" \
    --progress=plain \
    --build-arg "YB_VERSION=${YB_VERSION}" \
    --build-arg "YB_BUILD=${YB_BUILD}" \
    --build-arg "GO_VERSION=${GO_VERSION}" \
    -t "${IMAGE}" \
    -f docker/sandbox/Dockerfile \
    .

echo
echo "Built ${IMAGE}. Next: bash docker/sandbox/run.sh"

# voyager-sandbox: AI-agent playground with yb-voyager + Postgres 17 + YugabyteDB.
#
# - PG-only voyager build (no Oracle/MySQL stack).
# - Non-root `voyager` user; yugabyted's base_dir lives under $HOME/yb-data.
# - Repo is baked in at build time so `yb-voyager` works before the host bind mount
#   arrives, then the bind mount at /workspace/yb-voyager replaces it at runtime.
# - Network access is allowed at runtime (agents can go fetch docs, etc.).

FROM ubuntu:22.04

ARG TARGETARCH
ARG DEBIAN_FRONTEND=noninteractive
ARG YB_VERSION=2025.2.2.2
ARG YB_BUILD=b11
ARG GO_VERSION=1.24.2

# JAVA_HOME is a symlink created below so it's arch-agnostic (openjdk installs
# to /usr/lib/jvm/java-17-openjdk-{amd64,arm64} on Ubuntu).
ENV TZ=UTC \
    LANG=en_US.UTF-8 \
    LANGUAGE=en_US:en \
    LC_ALL=en_US.UTF-8 \
    JAVA_HOME=/opt/java17 \
    GOROOT=/usr/local/go \
    GOPATH=/home/voyager/go \
    YB_VOYAGER_SEND_DIAGNOSTICS=0

# 1. Base OS + build toolchain + agent conveniences. Also creates the
#    /opt/java17 symlink so later steps (and the voyager-env.sh profile) can
#    rely on one stable JAVA_HOME path regardless of host arch.
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
        ca-certificates curl wget gnupg lsb-release \
        sudo git rsync build-essential perl libdbi-perl jq tzdata locales \
        openjdk-17-jdk \
        vim less tree procps iproute2 net-tools \
 && ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone \
 && locale-gen en_US.UTF-8 \
 && ln -s "/usr/lib/jvm/java-17-openjdk-${TARGETARCH}" /opt/java17 \
 && rm -rf /var/lib/apt/lists/*

# 2. Go (pinned to match AGENTS.md). Installed to /usr/local/go so the voyager
#    installer's install_golang() check sees it and doesn't re-download.
RUN curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${TARGETARCH}.tar.gz" \
      | tar -C /usr/local -xz

ENV PATH=/opt/yugabyte/bin:$GOROOT/bin:$GOPATH/bin:$JAVA_HOME/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

# 3. Postgres 17 server + client from the PGDG repo.
RUN install -d /usr/share/postgresql-common/pgdg \
 && curl -fsSL https://www.postgresql.org/media/keys/ACCC4CF8.asc \
      | gpg --dearmor -o /etc/apt/trusted.gpg.d/pgdg.gpg \
 && echo "deb http://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" \
      > /etc/apt/sources.list.d/pgdg.list \
 && apt-get update \
 && apt-get install -y --no-install-recommends \
        postgresql-17 postgresql-client-17 \
 && rm -rf /var/lib/apt/lists/*

# 4. YugabyteDB tarball. The tarball naming differs between arches:
#    - amd64: yugabyte-<ver>-<build>-linux-x86_64.tar.gz
#    - arm64: yugabyte-<ver>-<build>-el8-aarch64.tar.gz
#    post_install.sh fixes up RPATHs / shebangs for the container's layout.
RUN set -eux; \
    case "${TARGETARCH}" in \
        amd64) YB_SUFFIX="linux-x86_64" ;; \
        arm64) YB_SUFFIX="el8-aarch64"  ;; \
        *) echo "Unsupported TARGETARCH=${TARGETARCH}"; exit 1 ;; \
    esac; \
    curl -fsSL "https://software.yugabyte.com/releases/${YB_VERSION}/yugabyte-${YB_VERSION}-${YB_BUILD}-${YB_SUFFIX}.tar.gz" \
      | tar -xz -C /opt \
 && mv "/opt/yugabyte-${YB_VERSION}" /opt/yugabyte \
 && /opt/yugabyte/bin/post_install.sh

# 5. Non-root user. yugabyted refuses to run when its data dir is root-owned
#    (see AGENTS.md), so both voyager and yugabyted run as `voyager`.
RUN useradd -m -s /bin/bash voyager \
 && echo "voyager ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/voyager \
 && chmod 0440 /etc/sudoers.d/voyager \
 && chown -R voyager:voyager /opt/yugabyte

# 6. Build voyager (PG-only) from the current repo checkout. The binary ends up
#    at /usr/local/bin/yb-voyager, the Debezium plugin at /opt/yb-voyager. A
#    runtime bind mount of the same repo will shadow /workspace/yb-voyager but
#    leaves these artifacts in place until the agent rebuilds with `-v`.
COPY --chown=voyager:voyager . /workspace/yb-voyager

USER voyager
WORKDIR /workspace/yb-voyager/installer_scripts

# Trust every path for git. The installer calls `git rev-parse` early, and at
# runtime the repo is bind-mounted with host UIDs that don't match `voyager`
# (uid 1000). This is safe because the container is a single-tenant sandbox.
RUN git config --global --add safe.directory '*'

RUN yes | ./install-yb-voyager -l -p

# 7. Entrypoint + profile so `docker exec -u voyager` inherits the same env.
USER root
COPY docker/sandbox/entrypoint.sh /usr/local/bin/entrypoint.sh
COPY docker/sandbox/voyager-env.sh /etc/profile.d/voyager-env.sh
RUN chmod +x /usr/local/bin/entrypoint.sh \
 && chown -R voyager:voyager /home/voyager

USER voyager
WORKDIR /workspace/yb-voyager

# Postgres 5432, YugabyteDB YSQL 5433, YB admin UIs 7000/9000/15433.
EXPOSE 5432 5433 7000 9000 15433

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
CMD ["sleep", "infinity"]

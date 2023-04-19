FROM golang:latest
RUN go install github.com/go-delve/delve/cmd/dlv@latest

FROM amd64/ubuntu:22.04
ARG DEBIAN_FRONTEND=noninteractive
ENV TZ=UTC
ENV MAVEN_URL="https://archive.apache.org/dist/maven/maven-3/3.8.4/binaries/apache-maven-3.8.4-bin.tar.gz"
ENV DEBEZIUM_COMMIT_HASH="4c892e5e3d5b4fad26daa4bf6e253cff0eb454a7"
ENV DEBEZIUM_VERSION="2.2.0-SNAPSHOT"
ENV debezium_server_dist_filename="debezium-server-dist-${DEBEZIUM_VERSION}.tar.gz"
RUN apt-get update && \
    apt-get install -y wget make && \
    wget https://s3.us-west-2.amazonaws.com/downloads.yugabyte.com/repos/reporpms/yb-apt-repo_1.0.0_all.deb && \
    apt-get install -y ./yb-apt-repo_1.0.0_all.deb && \
    apt-get update && \
    apt-get upgrade -y binutils && \
    apt-get install -y default-jdk &&\
    wget -nv ${MAVEN_URL} -O apache-maven-3.8.4-bin.tar.gz &&\
    tar -xzvf apache-maven-3.8.4-bin.tar.gz && \
    mv apache-maven-3.8.4 yb-debezium-maven-3.8.4 && \
    mkdir -p /opt/yb-voyager && \ 
    mv yb-debezium-maven-3.8.4 /opt/yb-voyager && \
    export PATH=/opt/yb-voyager/yb-debezium-maven-3.8.4/bin:$PATH && \
    rm apache-maven-3.8.4-bin.tar.gz &&\
    wget -nv https://github.com/yugabyte/debezium/archive/${DEBEZIUM_COMMIT_HASH}.tar.gz -O debezium.tar.gz && \
    tar -xzvf debezium.tar.gz && \
    cd debezium-${DEBEZIUM_COMMIT_HASH} && \
    sh buildDebeziumServer.sh && \
    tar -xvzf debezium-server/debezium-server-dist/target/${debezium_server_dist_filename} -C /opt/yb-voyager && \
    cd .. && \
    rm -r debezium-${DEBEZIUM_COMMIT_HASH} && \
    rm debezium.tar.gz && \
    apt-get install -y yb-voyager && \
    rm -rf /yb-apt-repo_1.0.0_all.deb && \
    apt-get clean
COPY --from=0 $HOME/go/bin/dlv /usr/local/bin
CMD ["yb-voyager"]

FROM perl:latest
RUN curl -L https://cpanmin.us | perl - --self-upgrade

FROM golang:latest
RUN go install github.com/go-delve/delve/cmd/dlv@latest

FROM amd64/ubuntu:22.04
ARG DEBIAN_FRONTEND=noninteractive
ENV TZ=UTC
ENV LANG en_US.UTF-8
ENV LANGUAGE en_US:en
ENV LC_ALL en_US.UTF-8
RUN ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone
RUN apt-get install -y tzdata
RUN apt-get update && \
    apt-get install -y wget curl openjdk-17-jre git sudo && \

    # wget https://s3.us-west-2.amazonaws.com/downloads.yugabyte.com/repos/reporpms/yb-apt-repo_1.0.0_all.deb && \
    # apt-get install -y ./yb-apt-repo_1.0.0_all.deb && \
    # apt-get update && \
    apt-get upgrade -y binutils && \
    curl -sL https://aka.ms/InstallAzureCLIDeb | bash && \
    git clone https://github.com/yugabyte/yb-voyager.git && \
    cd yb-voyager/installer_scripts && \
    echo "yes" | ./install-yb-voyager && \
    cat /tmp/install-yb-voyager.log && \
    source ~/.yb-voyager.rc && \
    # mkdir -p /opt/yb-voyager && \
    # apt-get install -y yb-voyager && \
    # rm -rf /yb-apt-repo_1.0.0_all.deb && \
    locale-gen en_US.UTF-8 && \
    apt-get clean
COPY --from=0 $HOME/go/bin/dlv /usr/local/bin
CMD [“yb-voyager version”]

FROM amd64/ubuntu:22.04

RUN apt-get update && \
    apt-get install -y wget && \
    wget https://s3.us-west-2.amazonaws.com/downloads.yugabyte.com/repos/reporpms/yb-apt-repo_1.0.0_all.deb && \
    apt-get install -y ./yb-apt-repo_1.0.0_all.deb && \ 
    apt-get clean && \
    apt-get update && \
    apt-get upgrade -y binutils && \
    (1 | apt-get upgrade -y tzdata) && \
    apt-get install -y yb-voyager && \
    rm -rf /yb-apt-repo_1.0.0_all.deb && \
    apt-get remove -y wget && \
    apt-get clean

CMD ["yb-voyager"]

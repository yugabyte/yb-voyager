FROM voyagerbase:1.0
ARG DEBIAN_FRONTEND=noninteractive
ENV TZ=UTC
ENV LANG en_US.UTF-8
ENV LANGUAGE en_US:en
ENV LC_ALL en_US.UTF-8
ENV CGO_ENABLED=1
ENV PATH="${PATH}:/root/go/bin/"

RUN git clone https://github.com/yugabyte/yb-voyager.git
RUN cd yb-voyager/yb-voyager && go build
RUN apt install -y vim
COPY install_binaries.sh /yb-voyager
RUN chmod 700 /yb-voyager/install_binaries.sh
RUN cd /yb-voyager && echo -e "Y\nY\nY" | ./install_binaries.sh

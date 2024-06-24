FROM voyagersrc:1.0
ARG DEBIAN_FRONTEND=noninteractive
ENV TZ=UTC
ENV LANG en_US.UTF-8
ENV LANGUAGE en_US:en
ENV LC_ALL en_US.UTF-8
ENV CGO_ENABLED=1
ARG VOYAGER_BRANCH_NAME

RUN cd yb-voyager/yb-voyager && git pull && git checkout $VOYAGER_BRANCH_NAME  && go clean && go build

RUN cp -r /yb-voyager/yb-voyager/src/srcdb/data/ /etc/yb-voyager/

ENV conf_file_name="/etc/yb-voyager/base-ora2pg.conf"
RUN mkdir -p /etc/yb-voyager
RUN cp /yb-voyager/yb-voyager/src/srcdb/data/sample-ora2pg.conf $conf_file_name
COPY sanity-py.py  /yb-voyager/yb-voyager
COPY pg_to_yb_env.sh /yb-voyager/yb-voyager
COPY mysql_to_yb_env.sh  /yb-voyager/yb-voyager
RUN ln -s /usr/bin/python3 /usr/bin/python

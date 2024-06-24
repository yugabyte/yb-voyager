FROM amd64/ubuntu:22.04
ARG DEBIAN_FRONTEND=noninteractive
ENV TZ=UTC
ENV LANG en_US.UTF-8
ENV LANGUAGE en_US:en
ENV LC_ALL en_US.UTF-8
ENV CGO_ENABLED=1
RUN apt-get update && apt-get install -y curl perl make libdbi-perl locales && \
    curl -L https://cpanmin.us | perl - --self-upgrade && \
    ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone && \
    apt-get install -y tzdata wget rsync openjdk-17-jre git sudo gcc && \
    apt-get upgrade -y binutils && \
    curl -sL https://aka.ms/InstallAzureCLIDeb | bash && \
    locale-gen en_US.UTF-8 && \
    apt-get remove -y gcc curl make wget git && \
    rm -rf /root/go /root/.cache /tmp/go && \
    apt-get clean
RUN apt-get install -y gcc 
RUN apt-get install -y git && apt install -y wget
RUN wget -nv https://golang.org/dl/go1.20.linux-amd64.tar.gz && \ 
    tar -C ~ -xzf go1.20.linux-amd64.tar.gz 
ENV PATH="${PATH}:/root/go/bin/"

# install pg client and pg dum
#RUN sh -c 'echo "deb http://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" > /etc/apt/sources.list.d/pgdg.list'
#RUN apt-get install curl
#RUN curl -fsSL https://www.postgresql.org/media/keys/ACCC4CF8.asc \ 
#  | gpg --batch --yes --dearmor -o /etc/apt/trusted.gpg.d/postgresql.gpg
#RUN apt update
#RUN apt install -y  postgresql-client-16

# install ora2pg installation

RUN apt install -y make
RUN apt update
RUN apt install -y mysql-client
RUN apt-get install -y libmysqlclient-dev
#RUN cpanm DBD::mysql@5.005 Test::NoWarnings@1.06 DBD::Oracle@1.83 1>&2
RUN cpanm DBD::mysql@5.005 Test::NoWarnings@1.06  1>&2


ENV ORA2PG_VERSION=${ORA2PG_VERSION:-"f08d47a4e5c20ff7d348e535ed05ab571b4bb350"}
RUN wget -nv https://github.com/darold/ora2pg/archive/${ORA2PG_VERSION}.tar.gz 1>&2
RUN tar -xzf ${ORA2PG_VERSION}.tar.gz 1>&2
RUN cd ora2pg-${ORA2PG_VERSION} && perl Makefile.PL && make && make install 
RUN cd ..   && rm -f ${ORA2PG_VERSION}.tar.gz


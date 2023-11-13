FROM ubuntu:devel

ENV DEBIAN_FRONTEND=noninteractive

# add our own overlay ppa for dependencies not yet in ubuntu:devel
RUN \
   apt-get update && \
   apt-get install -y software-properties-common && \
   add-apt-repository -y --ppa ppa:ubuntu-wsl-dev/ppa

FROM ubuntu:devel
COPY --from=0 /etc/apt/sources.list.d/* /etc/apt/sources.list.d/

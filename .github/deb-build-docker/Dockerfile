FROM ubuntu:mantic

ENV DEBIAN_FRONTEND=noninteractive

# add our own overlay ppa for dependencies not yet in ubuntu:devel.
# ensure we don’t use https to avoid having to install ca-certificates on the build system, which will
# skew the "no Internet access" test. The repo is still signed.
RUN \
   apt-get update && \
   apt-get install -y software-properties-common && \
   add-apt-repository -y --ppa ppa:ubuntu-wsl-dev/ppa && \
   sed -i "s#https#http#g" /etc/apt/sources.list.d/*

FROM ubuntu:mantic
COPY --from=0 /etc/apt/sources.list.d/* /etc/apt/sources.list.d/

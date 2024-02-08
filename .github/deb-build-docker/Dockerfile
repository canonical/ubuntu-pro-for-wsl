FROM ubuntu:devel

ENV DEBIAN_FRONTEND=noninteractive

# add our own overlay ppa for dependencies not yet in ubuntu:devel.
# ensure we don’t use https to avoid having to install ca-certificates on the build system, which will
# skew the "no Internet access" test. The repo is still signed.
#
# Installing python3-six is a temporary workaround for 
# https://bugs.launchpad.net/ubuntu/+source/software-properties/+bug/2049014
RUN \
   apt-get update && \
   apt-get install -y software-properties-common && \
   add-apt-repository -y --ppa ppa:ubuntu-wsl-dev/ppa && \
   sed -i "s#https#http#g" /etc/apt/sources.list.d/*

FROM ubuntu:devel
COPY --from=0 /etc/apt/sources.list.d/* /etc/apt/sources.list.d/

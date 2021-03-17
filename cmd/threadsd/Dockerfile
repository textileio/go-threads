FROM golang:1.16.0-buster
MAINTAINER Textile <contact@textile.io>

# This is (in large part) copied (with love) from
# https://hub.docker.com/r/ipfs/go-ipfs/dockerfile

# Install deps
RUN apt-get update && apt-get install -y \
  libssl-dev \
  ca-certificates

ENV SRC_DIR /go-threads

# Download packages first so they can be cached.
COPY go.mod go.sum $SRC_DIR/
RUN cd $SRC_DIR \
  && go mod download

COPY . $SRC_DIR

# Build the thing.
RUN cd $SRC_DIR \
  && BIN_BUILD_FLAGS="CGO_ENABLED=0 GOOS=linux" make build-threadsd

# Get su-exec, a very minimal tool for dropping privileges,
# and tini, a very minimal init daemon for containers
ENV SUEXEC_VERSION v0.2
ENV TINI_VERSION v0.19.0
RUN set -eux; \
    dpkgArch="$(dpkg --print-architecture)"; \
    case "${dpkgArch##*-}" in \
        "amd64" | "armhf" | "arm64") tiniArch="tini-static-$dpkgArch" ;;\
        *) echo >&2 "unsupported architecture: ${dpkgArch}"; exit 1 ;; \
    esac; \
  cd /tmp \
  && git clone https://github.com/ncopa/su-exec.git \
  && cd su-exec \
  && git checkout -q $SUEXEC_VERSION \
  && make su-exec-static \
  && cd /tmp \
  && wget -q -O tini https://github.com/krallin/tini/releases/download/$TINI_VERSION/$tiniArch \
  && chmod +x tini

# Now comes the actual target image, which aims to be as small as possible.
FROM busybox:1.31.1-glibc
LABEL maintainer="Textile <contact@textile.io>"

# Get the binary, entrypoint script, and TLS CAs from the build container.
ENV SRC_DIR /go-threads
COPY --from=0 $SRC_DIR/threadsd /usr/local/bin/threadsd
COPY --from=0 /tmp/su-exec/su-exec-static /sbin/su-exec
COPY --from=0 /tmp/tini /sbin/tini
COPY --from=0 /etc/ssl/certs /etc/ssl/certs

# This shared lib (part of glibc) doesn't seem to be included with busybox.
COPY --from=0 /lib/*-linux-gnu*/libdl.so.2 /lib/

# Copy over SSL libraries.
COPY --from=0 /usr/lib/*-linux-gnu*/libssl.so* /usr/lib/
COPY --from=0 /usr/lib/*-linux-gnu*/libcrypto.so* /usr/lib/

# hostAddr; should be exposed to the public
EXPOSE 4006
# apiAddr; can be exposed to the public
EXPOSE 5000
# apiProxyAddr; can be exposed to the public
EXPOSE 5050
# addrGateway; can be exposed to the public.
EXPOSE 8000

# Create the repo directory.
ENV THREADS_PATH /data/threads
RUN mkdir -p $THREADS_PATH \
  && adduser -D -h $THREADS_PATH -u 1000 -G users threads \
  && chown threads:users $THREADS_PATH

# Switch to a non-privileged user.
USER threads

# Expose the repo as a volume.
# Important this happens after the USER directive so permission are correct.
VOLUME $THREADS_PATH

ENTRYPOINT ["/sbin/tini", "--", "threadsd"]

CMD ["--badgerRepo=/data/threads"]

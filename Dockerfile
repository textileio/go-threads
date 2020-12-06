FROM golang:1.15.5-buster
MAINTAINER Textile <contact@textile.io>

# This is (in large part) copied (with love) from
# https://hub.docker.com/r/ipfs/go-ipfs/dockerfile

# Get source
ENV SRC_DIR /go-threads

# Download packages first so they can be cached.
COPY go.mod go.sum $SRC_DIR/
RUN cd $SRC_DIR \
  && go mod download

COPY . $SRC_DIR

# Install the daemon
RUN cd $SRC_DIR \
  && go install github.com/textileio/go-threads/threadsd

# Get su-exec, a very minimal tool for dropping privileges,
# and tini, a very minimal init daemon for containers
ENV SUEXEC_VERSION v0.2
ENV TINI_VERSION v0.16.1
RUN set -x \
  && cd /tmp \
  && git clone https://github.com/ncopa/su-exec.git \
  && cd su-exec \
  && git checkout -q $SUEXEC_VERSION \
  && make \
  && cd /tmp \
  && wget -q -O tini https://github.com/krallin/tini/releases/download/$TINI_VERSION/tini \
  && chmod +x tini

# Get the TLS CA certificates, they're not provided by busybox.
RUN apt-get update && apt-get install -y ca-certificates

# Now comes the actual target image, which aims to be as small as possible.
FROM busybox:1.31.0-glibc
LABEL maintainer="Textile <contact@textile.io>"

# Get the threads binary, entrypoint script, and TLS CAs from the build container.
ENV SRC_DIR /go-threads
COPY --from=0 /go/bin/threadsd /usr/local/bin/threadsd
COPY --from=0 /tmp/su-exec/su-exec /sbin/su-exec
COPY --from=0 /tmp/tini /sbin/tini
COPY --from=0 /etc/ssl/certs /etc/ssl/certs

# This shared lib (part of glibc) doesn't seem to be included with busybox.
COPY --from=0 /lib/x86_64-linux-gnu/libdl.so.2 /lib/libdl.so.2

# hostAddr; should be exposed to the public
EXPOSE 4006
# apiAddr; should *not* be exposed to the public unless intercepted by an auth system, e.g., textile
EXPOSE 6006
# apiProxyAddr; should *not* be exposed to the public unless intercepted by an auth system, e.g., textile
EXPOSE 6007

# Create the repo directory and switch to a non-privileged user.
ENV THREADS_PATH /data/threads
RUN mkdir -p $THREADS_PATH \
  && adduser -D -h $THREADS_PATH -u 1000 -G users textile \
  && chown -R textile:users $THREADS_PATH

# Switch to a non-privileged user
USER textile

# Expose the repo as a volume.
# Important this happens after the USER directive so permission are correct.
VOLUME $THREADS_PATH

ENTRYPOINT ["/sbin/tini", "--", "threadsd"]

CMD ["--repo=/data/threads"]

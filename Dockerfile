# BUILDER
FROM golang:1.19.0-bullseye as builder
WORKDIR /workspace

COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download

COPY Makefile Makefile
COPY src/ src/

RUN make test
RUN make build

ENV TINI_VERSION v0.19.0
ADD https://github.com/krallin/tini/releases/download/${TINI_VERSION}/tini-static /tini
RUN chmod +x /tini

# hadolint ignore=DL3008
RUN apt-get update && \
    apt-get -y --no-install-recommends install ca-certificates && \
    update-ca-certificates

# RUNTIME
FROM gcr.io/distroless/static-debian11:nonroot

WORKDIR /
COPY --from=builder /workspace/bin/mqtt-pinger /mqtt-pinger
COPY --from=builder /tini /tini
COPY --from=builder /etc/ssl/certs /etc/ssl/certs

ENTRYPOINT [ "/tini", "--", "/mqtt-pinger"]
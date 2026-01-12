FROM --platform=$BUILDPLATFORM golang:1.25.5-alpine3.23 AS builder

ENV GOSUMDB=off

WORKDIR /workspace

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

RUN go mod download

COPY pkg/ pkg/
COPY cmd/ cmd/

RUN go mod tidy

# Build
ARG TARGETOS TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o ./build/_output/bin/qubership-credential-manager \
    -gcflags all=-trimpath=${GOPATH} -asmflags all=-trimpath=${GOPATH} ./cmd/qubership-credential-manager

FROM alpine:3.22.1

ENV CONTROLLER=/usr/local/bin/qubership-credential-manager \
    USER_UID=1001

RUN echo "https://dl-cdn.alpinelinux.org/alpine/edge/main" > /etc/apk/repositories  \
    && echo "https://dl-cdn.alpinelinux.org/alpine/edge/community" >> /etc/apk/repositories \
    && apk add --update --upgrade --no-cache \
        libcurl curl

COPY --from=builder /workspace/build/_output/bin/qubership-credential-manager ${CONTROLLER}

USER ${USER_UID}

CMD ["sh", "-c", "${CONTROLLER}"]

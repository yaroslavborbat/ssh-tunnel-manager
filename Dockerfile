# Build the manager binary
FROM golang:1.23 AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
COPY go.mod go.mod
COPY go.sum go.sum

RUN go mod download

COPY cmd/ cmd/
COPY pkg/ pkg/

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o manager cmd/ssh-tunnel-manager/main.go

FROM alpine:3.21.0
RUN apk add --no-cache openssh expect
COPY scripts/ssh-add.exp /bin/
RUN chmod +x /bin/ssh-add.exp
WORKDIR /app
COPY --from=builder /workspace/manager /app/

ENTRYPOINT ["/app/manager"]

#FROM ubuntu
#RUN apt-get update -y && apt-get install openssh-client expect -y
#COPY scripts/ssh-add.exp /bin/
#RUN chmod +x /bin/ssh-add.exp
#WORKDIR /app
#COPY --from=builder /workspace/manager /app/
#
#ENTRYPOINT ["/app/manager"]
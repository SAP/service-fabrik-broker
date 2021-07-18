# Build the operator-apis binary
FROM golang:1.15 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY operator-apis/go.mod go.mod
COPY operator-apis/go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
COPY interoperator /interoperator
RUN go mod download

# Copy the go source
COPY operator-apis/main.go main.go
COPY operator-apis/internal/ internal/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o operator-apis main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
# FROM gcr.io/distroless/static:nonroot 
# Using alphine as xmake supports alphine seamlessly
FROM alpine:3.14.0

WORKDIR /
COPY --from=builder /workspace/operator-apis .

ENTRYPOINT ["/operator-apis"]

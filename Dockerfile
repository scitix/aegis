FROM golang:1.24.2 as builder
WORKDIR /aegis
RUN mkdir -p /collector
ADD . /aegis/
ADD manifests/collector/* /collector/
RUN cd /aegis \
  && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o aegiscli cmd/aegis-cli/main.go \
  && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o aegis cmd/aegis/main.go

FROM centos:8
WORKDIR /aegis
COPY --from=builder /aegis/aegiscli /aegis/
COPY --from=builder /aegis/aegis /aegis/
COPY --from=builder /collector /collector
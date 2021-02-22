FROM golang:1.15.8-buster as builder

WORKDIR /app
COPY . .

RUN make build

FROM alpine:latest
COPY --from=builder /app/bin/ilert-kube-agent /go/bin/ilert-kube-agent
CMD ["/go/bin/ilert-kube-agent"]

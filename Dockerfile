FROM golang:1.24.4-bookworm as builder

WORKDIR /app
COPY . .

RUN make build

FROM alpine:latest
COPY --from=builder /app/bin/ilert-kube-agent /bin/ilert-kube-agent
CMD ["/bin/ilert-kube-agent"]

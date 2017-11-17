FROM golang:1.9.0-alpine3.6 as build
WORKDIR /go/src/github.com/thomasjpfan/docker-scaler
COPY . .
RUN go build -o docker-scaler main.go

FROM alpine:3.6
RUN apk add --no-cache tini ca-certificates

COPY --from=build /go/src/github.com/thomasjpfan/docker-scaler/docker-scaler /usr/local/bin/docker-scaler
RUN chmod +x /usr/local/bin/docker-scaler

ENV MIN_SCALE_LABEL="com.df.scaleMin" \
    MAX_SCALE_LABEL="com.df.scaleMax" \
    SCALE_DOWN_BY_LABEL="com.df.scaleDownBy" \
    SCALE_UP_BY_LABEL="com.df.scaleUpBy" \
    DEFAULT_MIN_REPLICAS="1" \
    DEFAULT_MAX_REPLICAS="20" \
    ALERTMANAGER_ADDRESS="http://alertmanager:9093" \
    AWS_ENV_FILE="/run/secrets/aws" \
    AWS_DEFAULT_REGION="us-east-1" \
    AWS_MANAGER_GROUP_NAME="stack-NodeManagerConfig" \
    AWS_WORKER_GROUP_NAME="stack-NodeWorkerConfig" \
    DEFAULT_MIN_MANAGER_NODES="1" \
    DEFAULT_MAX_MANAGER_NODES="7" \
    DEFAULT_MIN_WORKER_NODES="1" \
    DEFAULT_MAX_WORKER_NODES="10" \
    DEFAULT_SCALE_DOWN_BY="1" \
    DEFAULT_SCALE_UP_BY="1"

EXPOSE 8080

ENTRYPOINT  ["/sbin/tini", "--"]
CMD ["docker-scaler"]

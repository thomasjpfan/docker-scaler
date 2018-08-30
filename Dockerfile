FROM golang:1.11.0-alpine3.8 as build
WORKDIR /develop
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o docker-scaler -ldflags '-w' -mod vendor main.go

FROM alpine:3.8
RUN apk add --no-cache tini ca-certificates

HEALTHCHECK --interval=5s CMD \
    wget --quiet --tries=1 --spider http://localhost:8080/v1/ping || exit 1

COPY --from=build /develop/docker-scaler /usr/local/bin/docker-scaler
RUN chmod +x /usr/local/bin/docker-scaler

ENV SERVER_PREFIX="/" \
    MIN_SCALE_LABEL="com.df.scaleMin" \
    MAX_SCALE_LABEL="com.df.scaleMax" \
    SCALE_DOWN_BY_LABEL="com.df.scaleDownBy" \
    SCALE_UP_BY_LABEL="com.df.scaleUpBy" \
    ALERT_SCALE_MIN="false" \
    ALERT_SCALE_MAX="true" \
    DEFAULT_MIN_REPLICAS="1" \
    DEFAULT_MAX_REPLICAS="5" \
    DEFAULT_SCALE_SERVICE_DOWN_BY="1" \
    DEFAULT_SCALE_SERVICE_UP_BY="1" \
    ALERTMANAGER_ADDRESS="" \
    ALERT_TIMEOUT="10" \
    RESCHEDULE_FILTER_LABEL="com.df.reschedule=true" \
    RESCHEDULE_TICKER_INTERVAL="60" \
    RESCHEDULE_TIMEOUT="1000" \
    RESCHEDULE_ENV_KEY="RESCHEDULE_DATE" \
    NODE_SCALER_BACKEND="" \
    ALERT_NODE_MIN="false" \
    ALERT_NODE_MAX="true" \
    DEFAULT_MIN_MANAGER_NODES="3" \
    DEFAULT_MAX_MANAGER_NODES="7" \
    DEFAULT_MIN_WORKER_NODES="0" \
    DEFAULT_MAX_WORKER_NODES="5" \
    AWS_ENV_FILE="/run/secrets/aws" \
    AWS_DEFAULT_REGION="us-east-1" \
    MIN_SCALE_MANAGER_NODE_LABEL="com.df.scaleManagerNodeMin" \
    MAX_SCALE_MANAGER_NODE_LABEL="com.df.scaleManagerNodeMax" \
    SCALE_MANAGER_NODE_DOWN_BY_LABEL="com.df.scaleManagerNodeDownBy" \
    SCALE_MANAGER_NODE_UP_BY_LABEL="com.df.scaleManagerNodeUpBy" \
    MIN_SCALE_WORKER_NODE_LABEL="com.df.scaleWorkerNodeMin" \
    MAX_SCALE_WORKER_NODE_LABEL="com.df.scaleWorkerNodeMax" \
    SCALE_WORKER_NODE_DOWN_BY_LABEL="com.df.scaleWorkerNodeDownBy" \
    SCALE_WORKER_NODE_UP_BY_LABEL="com.df.scaleWorkerNodeUpBy" \
    DEFAULT_MIN_MANAGER_NODES="3" \
    DEFAULT_MAX_MANAGER_NODES="7" \
    DEFAULT_MIN_WORKER_NODES="0" \
    DEFAULT_MAX_WORKER_NODES="5" \
    DEFAULT_SCALE_MANAGER_NODE_DOWN_BY="1" \
    DEFAULT_SCALE_MANAGER_NODE_UP_BY="1" \
    DEFAULT_SCALE_WORKER_NODE_DOWN_BY="1" \
    DEFAULT_SCALE_WORKER_NODE_UP_BY="1"

EXPOSE 8080

ENTRYPOINT  ["/sbin/tini", "--"]
CMD ["docker-scaler"]

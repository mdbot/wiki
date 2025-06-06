FROM golang:1.24.4 AS build
WORKDIR /go/src/app
COPY . .

RUN set -eux; \
    CGO_ENABLED=0 GO111MODULE=on go install .; \
    go run github.com/google/go-licenses@latest save ./... --save_path=/notices; \
    mkdir -p /mounts/data;

FROM ghcr.io/greboid/dockerbase/nonroot:1.20250326.0
COPY --from=build /go/bin/wiki /wiki
COPY --from=build /notices /notices
COPY --from=build --chown=65532:65532 /mounts /
VOLUME /data
WORKDIR /
ENTRYPOINT ["/wiki"]
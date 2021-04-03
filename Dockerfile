FROM golang:1.16 AS build

# Build the app
WORKDIR /app
COPY . /app
RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"' -o /go/bin/app .
RUN mkdir /data

# Generate licence information
RUN go get github.com/google/go-licenses && go-licenses save ./... --save_path=/notices

FROM gcr.io/distroless/static:nonroot

LABEL org.opencontainers.image.source="https://github.com/mdbot/wiki"

COPY --from=build /go/bin/app /wiki
COPY --from=build /notices /notices
COPY --from=build --chown=nonroot /data /data
VOLUME /data
WORKDIR /
CMD ["/wiki"]

FROM golang:1.24 AS build

# Build the app
WORKDIR /app
COPY . /app
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -gcflags=./dontoptimizeme=-N -ldflags=-s -o /go/bin/app .
RUN mkdir /data

# Generate licence information - Ignore some valid licenses 
RUN go run github.com/google/go-licenses@latest save ./... --save_path=/notices --ignore github.com/pjbgf/sha1cd/cgo --ignore github.com/cloudflare/circl --ignore golang.org/x

FROM gcr.io/distroless/static:nonroot

COPY --from=build /go/bin/app /wiki
COPY --from=build /notices /notices
COPY --from=build /etc/mime.types /etc/mime.types
COPY --from=build --chown=nonroot /data /data
VOLUME /data
WORKDIR /
CMD ["/wiki"]

FROM golang:1.16 AS build
WORKDIR /app
COPY . /app
RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"' -o /go/bin/app .

FROM gcr.io/distroless/base-debian10
COPY --from=build /go/bin/app /wiki
CMD ["/wiki"]

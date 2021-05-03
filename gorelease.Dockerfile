FROM golang:1.16 AS build
RUN mkdir /data

FROM gcr.io/distroless/static:nonroot

COPY --from=build --chown=nonroot /data /data
COPY --from=build /etc/mime.types /etc/mime.types

COPY binaries/wiki_linux_arm64* /wiki
COPY notices /notices

VOLUME /data
WORKDIR /
CMD ["/wiki"]

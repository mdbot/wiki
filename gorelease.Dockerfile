FROM golang:1.16 AS build
RUN mkdir /data

FROM gcr.io/distroless/static:nonroot

COPY --from=build --chown=nonroot /data /data

COPY wiki /wiki
COPY notices /notices

VOLUME /data
WORKDIR /
CMD ["/wiki"]
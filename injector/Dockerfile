FROM golang:1.21.5 as builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY injector injector
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /out/injector ./injector/cmd

FROM scratch
ARG LINKERD_VERSION
ENV LINKERD_CONTAINER_VERSION_OVERRIDE=${LINKERD_VERSION}
COPY --from=builder /out/injector /injector

ENTRYPOINT ["/injector"]

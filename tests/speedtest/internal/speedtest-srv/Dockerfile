FROM golang:1.21 AS builder

WORKDIR /go/speedtest-srv
COPY ../speedtest-srv .
RUN go mod init speedtest
RUN go mod download
RUN CGO_ENABLED=0 go build -trimpath -o /speedtest

FROM ghcr.io/danieletorelli/librespeed-cli
WORKDIR /
COPY --from=builder /speedtest .
EXPOSE 8080

ENTRYPOINT ["./speedtest"]
FROM golang:1.21 AS builder

WORKDIR /go/speedtest-srv
COPY . .
RUN go mod init speedtest
RUN go get
RUN CGO_ENABLED=0 go build -tags docker -trimpath -o /speedtest

FROM ghcr.io/danieletorelli/librespeed-cli
WORKDIR /
COPY --from=builder /speedtest .
EXPOSE 8080

ENTRYPOINT ["./speedtest"]
FROM golang:1.14 as builder

COPY . /src
WORKDIR /src
RUN go env -w GO111MODULE=on
RUN go env -w GOPROXY=https://goproxy.cn,direct
RUN go env -w GOINSECURE=git.ctyun.cn
RUN go env -w GOPRIVATE=git.ctyun.cn
RUN go mod download
RUN go build -tags netgo

FROM alpine:3.12
COPY --from=builder ./src/go-vncproxy /
COPY --from=builder ./src/etc/app.yml /etc/app.yml
RUN chmod +X ./go-vncproxy
CMD ["./go-vnc"]

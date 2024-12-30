FROM docker.io/library/golang:1.23 as builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN make build-static

FROM scratch

COPY --from=builder /app/dist/gdu /opt/gdu

ENTRYPOINT ["/opt/gdu"]
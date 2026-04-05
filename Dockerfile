FROM golang:1.26-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w" \
    -o /schnueffelstueck \
    ./cmd/schnueffelstueck

FROM scratch
COPY --from=builder /schnueffelstueck /schnueffelstueck
ENTRYPOINT ["/schnueffelstueck"]

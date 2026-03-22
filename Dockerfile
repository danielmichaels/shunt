FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder

ARG TARGETARCH
ARG VERSION=dev

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w -X main.version=${VERSION}" -o /shunt ./cmd/shunt

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /shunt /shunt

EXPOSE 2112 8080

ENTRYPOINT ["/shunt"]

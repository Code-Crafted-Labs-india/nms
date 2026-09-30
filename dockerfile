FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/nms-middleware .

FROM alpine:3.22
RUN apk add --no-cache ca-certificates \
    && addgroup -S -g 10001 nms \
    && adduser -S -D -H -u 10001 -G nms nms
COPY --from=build /out/nms-middleware /usr/local/bin/nms-middleware
USER 10001:10001
EXPOSE 8080
CMD ["/usr/local/bin/nms-middleware", "-env", "production"]

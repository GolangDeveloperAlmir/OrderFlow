FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o orderflow ./cmd/api

FROM alpine:3.19
WORKDIR /app
COPY --from=build /src/orderflow /app/orderflow
RUN mkdir -p /app/certs
EXPOSE 8443
CMD ["/app/orderflow"]

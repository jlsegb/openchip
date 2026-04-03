FROM golang:1.22-bookworm AS build
WORKDIR /app

COPY api/ ./api
COPY db ./db
WORKDIR /app/api
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/openchip-api ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /bin/openchip-api /app/openchip-api
COPY db /app/db
ENV PORT=8080
USER nonroot:nonroot
CMD ["/app/openchip-api"]

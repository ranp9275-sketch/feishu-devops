FROM docker.1ms.run/golang:1.23 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o devops ./

FROM gcr.io/distroless/base-debian12:nonroot
WORKDIR /app
ENV PORT=8080
COPY --from=builder /app/devops /app/devops
EXPOSE 8080
USER nonroot
ENTRYPOINT ["/app/devops"]


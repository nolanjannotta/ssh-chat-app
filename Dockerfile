# syntax=docker/dockerfile:1

FROM golang:latest AS builder

WORKDIR /build


COPY . . 

# COPY ~/.ssh/chat=app /.ssh/chat-app



RUN go mod download
RUN GOOS=linux go build -o . .

FROM gcr.io/distroless/base-debian12


WORKDIR /app
COPY --from=builder /build/chat-app .
# COPY --from=builder /.ssh/chat-app /.ssh/chat-app



EXPOSE 2227





CMD ["/app/chat-app"]


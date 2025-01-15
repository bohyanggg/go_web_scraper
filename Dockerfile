# Use Go official image for building
FROM golang:1.23 as builder

# Set the working directory
WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application for x86_64 architecture
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o scraper

# Use a lightweight image for the final stage
FROM alpine:latest

# Install necessary certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Set the working directory
WORKDIR /root/

# Copy the built application from the builder
COPY --from=builder /app/scraper .

# Command to run the application
CMD ["./scraper"]

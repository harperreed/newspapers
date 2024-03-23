# Use the official Go image as the base image
FROM golang:1.16

# Set the working directory inside the container
WORKDIR /app

# Copy the Go module files
COPY go.mod go.sum ./

# Download the Go module dependencies
RUN go mod download

# Copy the application source code
COPY . .

# Build the Go application
RUN go build -o main .

# Expose the port on which the application will run (adjust if necessary)
EXPOSE 8080

# Set the entry point for the container
CMD ["./main"]

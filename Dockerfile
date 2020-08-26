FROM golang:alpine

# Set necessary environmet variables needed for our image
ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

# Move to working directory /build
WORKDIR /src

# Copy and download dependency using go mod
COPY /src/go.mod .
COPY /src/go.sum .
RUN go mod download

# Copy the code into the container
COPY . .

# Build the application
RUN go build -v.
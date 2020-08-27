FROM alpine:3.12
RUN apk update && apk add go gcc bash musl-dev openssl-dev ca-certificates && update-ca-certificates
RUN apk add git
ARG GOLANG_VERSION=1.14.3
RUN wget https://dl.google.com/go/go$GOLANG_VERSION.src.tar.gz && tar -C /usr/local -xzf go$GOLANG_VERSION.src.tar.gz

RUN cd /usr/local/go/src && ./make.bash

ENV PATH=$PATH:/usr/local/go/bin

RUN rm go$GOLANG_VERSION.src.tar.gz

#we delete the apk installed version to avoid conflict
RUN apk del go


# Set necessary environmet variables needed for our image
ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /data

RUN git clone https://github.com/akorwash/QuizBattle.git /data/app
 
# Move to working directory /build
WORKDIR /data/app/src


# Build the application
RUN go build -o dist
RUN ls  /data/app/src
# Export necessary port
EXPOSE 8080

# Command to run when starting the container
CMD ["/data/app/src/dist"]

# Production Dockerfile supporting multi-architecture (AMD64 & ARM64)
FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata curl

ARG TARGETARCH=amd64

COPY bin/linux/${TARGETARCH}/chordd /usr/local/bin/chordd
COPY bin/linux/${TARGETARCH}/chordcli /usr/local/bin/chordcli
RUN chmod +x /usr/local/bin/chordd /usr/local/bin/chordcli

EXPOSE 8545 26657 9090 26656
ENTRYPOINT ["chordd"]
CMD ["--port=8545"]

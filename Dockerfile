FROM golang:1.26.6-alpine3.24@sha256:af8d6740070b8906d12eae1c3e3ea0957fb63f492051ea05e354c38ef9fe88df AS builder

WORKDIR /src

COPY src/go.mod src/go.sum ./
RUN go mod download && go mod verify

COPY src/ ./
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/quizbattle .

FROM gcr.io/distroless/static-debian12:nonroot@sha256:1b7b9f0f0e0a1d2155f531db587cc48ec26aaf97ab64364225f5bf18a054e66a

WORKDIR /app

COPY --from=builder --chown=65532:65532 /out/quizbattle /app/quizbattle
COPY --from=builder --chown=65532:65532 /src/api/view /app/api/view
COPY --from=builder --chown=65532:65532 /src/static /app/static
COPY --chown=65532:65532 data/question-bank/questions.ar.jsonl /app/data/question-bank/questions.ar.jsonl

EXPOSE 8080

USER 65532:65532
ENTRYPOINT ["/app/quizbattle"]

# ---------- Build deps ----------
FROM golang:1.19-alpine AS deps
RUN apk add --no-cache git
WORKDIR /workspace
COPY go.mod go.sum ./
RUN go mod download

# ---------- Build ----------
FROM deps AS build
# Build for the requested target (set by buildx)
ARG TARGETOS
ARG TARGETARCH
ENV CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH}

COPY . .
# smaller, reproducible binary
RUN go build -trimpath -buildvcs=false \
    -ldflags='-s -w -extldflags "-static"' \
    -o /out/webhook .

# ---------- Runtime ----------
FROM alpine:3.19
RUN apk add --no-cache ca-certificates libcap
COPY --from=build /out/webhook /usr/local/bin/webhook
# allow binding to :443 as non-root
RUN setcap 'cap_net_bind_service=+ep' /usr/local/bin/webhook

USER 1001
ENTRYPOINT ["/usr/local/bin/webhook"]

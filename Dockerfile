# ============================================================
# Stage 1: Build the summer binary
# ============================================================
FROM golang:1.25.7-alpine AS builder

RUN apk add --no-cache git make

WORKDIR /src

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN make build

# ============================================================
# Stage 2: Minimal runtime image
# ============================================================
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

# Copy binary
COPY --from=builder /src/build/summer /usr/local/bin/summer

# Copy builtin skills
COPY --from=builder /src/skills /opt/summer/skills

# Create summer home directory
RUN mkdir -p /root/.summer/workspace/skills && \
    cp -r /opt/summer/skills/* /root/.summer/workspace/skills/ 2>/dev/null || true

ENTRYPOINT ["summer"]
CMD ["gateway"]

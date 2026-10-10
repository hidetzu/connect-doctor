# ConnectDoctor on Cloud Run (docs/adr/0008, docs/DEPLOY.md).
#
# ⚠ The runtime image must carry CA certificates: the TLS step trusts the
#   system root store and nothing else (docs/DESIGN.md § 2, TLS).
#   distroless/static ships /etc/ssl/certs/ca-certificates.crt.
# ⚠ Standard library only (docs/adr/0006): no module download happens here.

FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /connect-doctor ./cmd/connect-doctor

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /connect-doctor /connect-doctor
USER nonroot:nonroot
# Cloud Run sets PORT; without -addr the binary listens on 0.0.0.0:$PORT.
ENTRYPOINT ["/connect-doctor"]

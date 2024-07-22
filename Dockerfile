FROM golang:1.22 AS build-stage

# Build the Go application runtime.

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . ./

RUN CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=$(git describe --tags --always --dirty)" ./cmd/runtime

FROM ubuntu:latest

# Install Nix.

# https://github.com/DeterminateSystems/nix-installer?tab=readme-ov-file#in-a-container
RUN apt update 
RUN apt -y install curl xz-utils sudo git vim

# TODO: Pin the version, and check the hash.
RUN curl --proto '=https' --tlsv1.2 -sSf -L https://install.determinate.systems/nix | sh -s -- install linux \
  --extra-conf "sandbox = false" \
  --init none \
  --no-confirm
ENV PATH="${PATH}:/nix/var/nix/profiles/default/bin"

# Expect code to be mounted in the code directory.
WORKDIR /code
# Configure git not to prevent git operations in the /code directory, since this is
# where source code will be mounted.
RUN git config --global --add safe.directory /code

# Copy the runtime app.
COPY --from=build-stage /app/runtime /usr/local/bin/runtime

ENTRYPOINT [ "/usr/local/bin/runtime" ]

FROM golang:1.23 AS build-stage

# Build the Go application runtime.

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . ./

RUN CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=$(git describe --tags --always --dirty)" -o /app/validatecmd ./cmd/validate
RUN CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=$(git describe --tags --always --dirty)" -o /app/flakegapcmd ./cmd/flakegap

FROM ubuntu:latest AS deps

ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update
RUN apt-get -y install wget

# Download installers.
RUN mkdir /deps
WORKDIR /deps
COPY deps.sha256sum deps.sha256sum
RUN wget https://github.com/DeterminateSystems/nix-installer/releases/download/v3.5.2/nix-installer-`arch`-linux
RUN sha256sum -c deps.sha256sum --ignore-missing
RUN mv nix-installer-`arch`-linux nix-installer
RUN ls /deps

FROM ubuntu:latest

# Install Nix.

ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update
RUN apt-get install -y curl xz-utils sudo git vim

RUN mkdir /deps
COPY --from=deps /deps/nix-installer /deps/nix-installer

# https://github.com/DeterminateSystems/nix-installer?tab=readme-ov-file#in-a-container
# https://github.com/DeterminateSystems/nix-installer/issues/324#issuecomment-1491888268
RUN chmod +x /deps/nix-installer
RUN /deps/nix-installer install linux \
  --extra-conf "filter-syscalls = false" \
  --init none \
  --no-confirm
ENV PATH="${PATH}:/nix/var/nix/profiles/default/bin"

# Enable kvm in Nix.
RUN echo "system-features = nixos-test benchmark big-parallel kvm" >> /etc/nix/nix.conf

# Expect code to be mounted in the code directory.
WORKDIR /code
# Configure git not to prevent git operations in the /code directory, since this is
# where source code will be mounted.
RUN git config --global --add safe.directory /code

# Copy the validate app.
COPY --from=build-stage /app/validatecmd /usr/local/bin/validate
COPY --from=build-stage /app/flakegapcmd /usr/local/bin/flakegap

ENTRYPOINT [ "/usr/local/bin/validate" ]

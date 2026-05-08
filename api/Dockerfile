FROM golang:1.25-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/api ./cmd/api

# Runtime: needs terraform binary and ansible-runner
FROM debian:bookworm-slim
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates curl unzip python3 python3-pip openssh-client bind9-dnsutils && \
    rm -rf /var/lib/apt/lists/* && \
    pip3 install --break-system-packages ansible-runner ansible
ARG TF_VERSION=1.7.5
RUN curl -fsSL "https://releases.hashicorp.com/terraform/${TF_VERSION}/terraform_${TF_VERSION}_linux_amd64.zip" -o /tmp/tf.zip \
 && unzip /tmp/tf.zip -d /usr/local/bin && rm /tmp/tf.zip
# Install kubectl pinned to the k3s server's k8s minor (v1.31).
# kubectl is feature-compatible across +/- 1 minor version, so this works for
# v1.30/v1.31/v1.32 clusters without rebuilding the image.
RUN curl -fsSL -o /usr/local/bin/kubectl https://dl.k8s.io/release/v1.31.12/bin/linux/amd64/kubectl && \
    chmod +x /usr/local/bin/kubectl && \
    kubectl version --client=true
# Install helm pinned to v3.16.x for chart install/upgrade/uninstall.
RUN curl -fsSL https://get.helm.sh/helm-v3.16.4-linux-amd64.tar.gz | tar -xz -C /tmp \
    && mv /tmp/linux-amd64/helm /usr/local/bin/helm \
    && rm -rf /tmp/linux-amd64 \
    && helm version --short
COPY --from=build /out/api /usr/local/bin/api
WORKDIR /var/lib/bandolier
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/api"]

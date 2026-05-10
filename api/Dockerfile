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
# Terraform — verify SHA256SUMS published alongside the release before
# unzipping. A tampered binary served from the upstream CDN won't pass.
ARG TF_VERSION=1.7.5
RUN cd /tmp && \
    curl -fsSLO "https://releases.hashicorp.com/terraform/${TF_VERSION}/terraform_${TF_VERSION}_linux_amd64.zip" && \
    curl -fsSLO "https://releases.hashicorp.com/terraform/${TF_VERSION}/terraform_${TF_VERSION}_SHA256SUMS" && \
    grep "  terraform_${TF_VERSION}_linux_amd64.zip\$" "terraform_${TF_VERSION}_SHA256SUMS" | sha256sum -c - && \
    unzip "terraform_${TF_VERSION}_linux_amd64.zip" -d /usr/local/bin && \
    rm "terraform_${TF_VERSION}_linux_amd64.zip" "terraform_${TF_VERSION}_SHA256SUMS"

# kubectl pinned to the k3s server's k8s minor (v1.31). kubectl is feature-
# compatible across ±1 minor version, so this works for v1.30/v1.31/v1.32
# clusters without rebuilding the image. The .sha256 file beside the binary
# contains just the hash; we construct the sha256sum -c input ourselves.
ARG KUBECTL_VERSION=v1.31.12
RUN curl -fsSL -o /usr/local/bin/kubectl "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl" && \
    EXPECTED="$(curl -fsSL "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl.sha256")" && \
    echo "${EXPECTED}  /usr/local/bin/kubectl" | sha256sum -c - && \
    chmod +x /usr/local/bin/kubectl && \
    kubectl version --client=true

# helm pinned to v3.16.x for chart install/upgrade/uninstall. The .sha256sum
# file is in standard `<hash>  <filename>` format.
ARG HELM_VERSION=v3.16.4
RUN cd /tmp && \
    curl -fsSLO "https://get.helm.sh/helm-${HELM_VERSION}-linux-amd64.tar.gz" && \
    curl -fsSLO "https://get.helm.sh/helm-${HELM_VERSION}-linux-amd64.tar.gz.sha256sum" && \
    sha256sum -c "helm-${HELM_VERSION}-linux-amd64.tar.gz.sha256sum" && \
    tar -xz -f "helm-${HELM_VERSION}-linux-amd64.tar.gz" && \
    mv linux-amd64/helm /usr/local/bin/helm && \
    rm -rf linux-amd64 "helm-${HELM_VERSION}-linux-amd64.tar.gz" "helm-${HELM_VERSION}-linux-amd64.tar.gz.sha256sum" && \
    helm version --short
COPY --from=build /out/api /usr/local/bin/api
WORKDIR /var/lib/bandolier
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/api"]

FROM registry.suse.com/bci/bci-base:15.5
ARG TARGETPLATFORM
ARG TARGETARCH
ARG TARGETOS

ENV TARGETPLATFORM=${TARGETPLATFORM:-"linux/amd64"} ARCH=${TARGETARCH:-"amd64"} OS=${TARGETOS:-"linux"}
ENV KUBE_EXPLORER_VERSION=v0.4.0
ENV HELM_DASHBOARD_VERSION=1.3.3

RUN zypper -n install curl ca-certificates tar gzip
RUN mkdir /home/shell && \
    echo '. /etc/profile.d/bash_completion.sh' >> /home/shell/.bashrc && \
    echo 'alias k="kubectl"' >> /home/shell/.bashrc && \
    echo 'source <(kubectl completion bash)' >> /home/shell/.bashrc && \
    echo 'PS1="> "' >> /home/shell/.bashrc

RUN curl -sSL https://github.com/cnrancher/kube-explorer/releases/download/${KUBE_EXPLORER_VERSION}/kube-explorer-${OS}-${ARCH} > /usr/local/bin/kube-explorer && \
    chmod +x /usr/local/bin/kube-explorer

RUN curl -sLf https://github.com/komodorio/helm-dashboard/releases/download/v${HELM_DASHBOARD_VERSION}/helm-dashboard_${HELM_DASHBOARD_VERSION}_Linux_x86_64.tar.gz | tar xvzf - -C /usr/local/bin && \
    chmod +x /usr/local/bin/helm-dashboard

ENV AUTOK3S_CONFIG=/root/.autok3s
ENV DOCKER_HOST=unix:///var/run/docker.sock
ENV HOME=/root
ENV AUTOK3S_HELM_DASHBOARD_ADDRESS=0.0.0.0

WORKDIR /home/shell
VOLUME /root/.autok3s
COPY bin/${TARGETPLATFORM}/autok3s /usr/local/bin/autok3s
RUN ln -sf autok3s /usr/local/bin/kubectl
ENTRYPOINT ["autok3s"]
CMD ["serve", "--bind-address=0.0.0.0"]

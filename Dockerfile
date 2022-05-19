FROM registry.suse.com/bci/bci-base:15.3
ARG TARGETPLATFORM
ARG TARGETARCH
ARG TARGETOS

ENV TARGETPLATFORM=${TARGETPLATFORM:-"linux/amd64"} ARCH=${TARGETARCH:-"amd64"} OS=${TARGETOS:-"linux"}
ENV KUBE_EXPLORER_VERSION=v0.2.9

RUN zypper -n install curl ca-certificates
RUN mkdir /home/shell && \
    echo '. /etc/profile.d/bash_completion.sh' >> /home/shell/.bashrc && \
    echo 'alias k="kubectl"' >> /home/shell/.bashrc && \
    echo 'source <(kubectl completion bash)' >> /home/shell/.bashrc && \
    echo 'PS1="> "' >> /home/shell/.bashrc

RUN curl -sSL https://github.com/cnrancher/kube-explorer/releases/download/${KUBE_EXPLORER_VERSION}/kube-explorer-${OS}-${ARCH} > /usr/local/bin/kube-explorer && \
    chmod +x /usr/local/bin/kube-explorer

ENV AUTOK3S_CONFIG /root/.autok3s
ENV HOME /root

WORKDIR /home/shell
VOLUME /root/.autok3s
COPY bin/${TARGETPLATFORM}/autok3s /usr/local/bin/autok3s
RUN ln -sf autok3s /usr/local/bin/kubectl
ENTRYPOINT ["autok3s"]
CMD ["serve", "--bind-address=0.0.0.0"]

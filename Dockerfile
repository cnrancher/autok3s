FROM --platform=$TARGETPLATFORM alpine

# NB(thxCode): automatic platform ARGs, ref to:
# - https://docs.docker.com/engine/reference/builder/#automatic-platform-args-in-the-global-scope
ARG TARGETPLATFORM
ARG TARGETOS
ARG TARGETARCH

RUN apk add -U --no-cache bash bash-completion curl wget
RUN mkdir /home/shell && \
    echo '. /etc/profile.d/bash_completion.sh' >> /home/shell/.bashrc && \
    echo 'alias k="kubectl"' >> /home/shell/.bashrc && \
    echo 'source <(kubectl completion bash)' >> /home/shell/.bashrc && \
    echo 'PS1="> "' >> /home/shell/.bashrc

RUN wget -O /usr/local/bin/kube-explorer https://github.com/cnrancher/kube-explorer/releases/download/v0.1.3/kube-explorer-${TARGETOS}-${TARGETARCH} && \
    chmod +x /usr/local/bin/kube-explorer

ENV AUTOK3S_CONFIG /root/.autok3s
ENV HOME /root
ENV K3D_HELPER_IMAGE_TAG v4.4.7

WORKDIR /home/shell
VOLUME /root/.autok3s
COPY bin/autok3s_${TARGETOS}_${TARGETARCH} /usr/local/bin/autok3s
RUN ln -sf autok3s /usr/local/bin/kubectl
ENTRYPOINT ["autok3s"]
CMD ["serve", "--bind-address=0.0.0.0"]

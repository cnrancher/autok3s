FROM registry.suse.com/suse/sle15:15.3

ARG ARCH=amd64

RUN zypper -n install curl wget ca-certificates
RUN mkdir /home/shell && \
    echo '. /etc/profile.d/bash_completion.sh' >> /home/shell/.bashrc && \
    echo 'alias k="kubectl"' >> /home/shell/.bashrc && \
    echo 'source <(kubectl completion bash)' >> /home/shell/.bashrc && \
    echo 'PS1="> "' >> /home/shell/.bashrc

RUN wget -O /usr/local/bin/kube-explorer https://github.com/cnrancher/kube-explorer/releases/download/v0.2.6/kube-explorer-linux-${ARCH} && \
    chmod +x /usr/local/bin/kube-explorer

ENV AUTOK3S_CONFIG /root/.autok3s
ENV HOME /root

WORKDIR /home/shell
VOLUME /root/.autok3s
COPY bin/autok3s_linux_${ARCH} /usr/local/bin/autok3s
RUN ln -sf autok3s /usr/local/bin/kubectl
ENTRYPOINT ["autok3s"]
CMD ["serve", "--bind-address=0.0.0.0"]

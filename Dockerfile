FROM --platform=$TARGETPLATFORM alpine

# NB(thxCode): automatic platform ARGs, ref to:
# - https://docs.docker.com/engine/reference/builder/#automatic-platform-args-in-the-global-scope
ARG TARGETPLATFORM
ARG TARGETOS
ARG TARGETARCH

RUN apk add -U --no-cache bash bash-completion
RUN mkdir /home/shell && \
    echo '. /etc/profile.d/bash_completion.sh' >> /home/shell/.bashrc && \
    echo 'alias k="autok3s kubectl"' >> /home/shell/.bashrc && \
    echo 'alias kubectl="autok3s kubectl"' >> /home/shell/.bashrc && \
    echo 'source <(autok3s kubectl completion bash)' >> /home/shell/.bashrc && \
    echo 'PS1="> "' >> /home/shell/.bashrc

WORKDIR /home/shell
VOLUME /var/lib/autok3s
COPY bin/autok3s_${TARGETOS}_${TARGETARCH} /usr/local/bin/autok3s
ENTRYPOINT ["autok3s"]
CMD ["serve", "--bind-address=0.0.0.0"]

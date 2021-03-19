FROM --platform=$TARGETPLATFORM alpine

# NB(thxCode): automatic platform ARGs, ref to:
# - https://docs.docker.com/engine/reference/builder/#automatic-platform-args-in-the-global-scope
ARG TARGETPLATFORM
ARG TARGETOS
ARG TARGETARCH

WORKDIR /
VOLUME /var/lib/autok3s
COPY bin/autok3s_${TARGETOS}_${TARGETARCH} /autok3s
ENTRYPOINT ["/autok3s"]

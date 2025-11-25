FROM docker.io/linuxsuren/api-testing:v0.0.14
WORKDIR /workspace
COPY testcase.yaml .
COPY data data
COPY entrypoint.sh entrypoint.sh
CMD [ "/workspace/entrypoint.sh" ]

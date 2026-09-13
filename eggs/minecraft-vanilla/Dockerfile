FROM eclipse-temurin:21-jre-jammy

RUN useradd -m -u 1000 oikos
WORKDIR /home/oikos/server
USER oikos

COPY --chown=oikos:oikos server.jar ./server.jar

# CMD tidak didefinisikan di sini — startup command diambil dari egg.yaml
# agar tetap fleksibel untuk variable substitution ({{MIN_MEMORY}}, dst)

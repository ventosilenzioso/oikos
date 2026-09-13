FROM eclipse-temurin:21-jre-jammy

RUN useradd -m -u 1000 oikos
WORKDIR /home/oikos/server
USER oikos

COPY --chown=oikos:oikos server.jar ./server.jar

# CMD is intentionally omitted; the startup command comes from egg.yaml
# so variable substitution ({{MIN_MEMORY}}, etc.) remains flexible.

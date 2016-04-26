# This Dockerfile creates a minimal image. Check out build.sh on how to build
# the prerequisites for this.
FROM scratch

COPY ca-certificates.crt /etc/ssl/certs/

WORKDIR /app/user

COPY url-shortener ./
COPY public ./public/

EXPOSE 5000

ENTRYPOINT ["./url-shortener"]

# Label the image with the git commit.
ARG GIT_SHA1=non-git
LABEL net.corpet.git=$GIT_SHA1

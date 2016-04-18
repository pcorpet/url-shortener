# This Dockerfile creates a minimal image. Check out build.sh on how to build
# the prerequisites for this.
FROM scratch

ADD ca-certificates.crt /etc/ssl/certs/

WORKDIR /app/user

ADD url-shortener public ./

EXPOSE 5000

ENTRYPOINT ["./url-shortener"]

# Label the image with the git commit.
ARG GIT_SHA1=non-git
LABEL net.corpet.git=$GIT_SHA1

FROM scratch

COPY url-shortener /app/user/
COPY ca-certificates.crt /etc/ssl/certs/
ADD public /app/user/public

WORKDIR /app/user

EXPOSE 5000

ENTRYPOINT ["/app/user/url-shortener"]

FROM heroku/cedar

COPY url-shortener /app/user/
RUN mkdir -p /app/user/public
ADD public /app/user/public

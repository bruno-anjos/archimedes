FROM busybox
COPY ./archimedes /archimedes

EXPOSE 50000

CMD ["/archimedes", "-d"]
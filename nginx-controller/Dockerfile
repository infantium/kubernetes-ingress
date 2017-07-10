FROM alpine:3.6

# Add nginx
RUN apk add --update nginx

# forward nginx access and error logs to stdout and stderr of the ingress
# controller process
RUN ln -sf /proc/1/fd/1 /var/log/nginx/access.log \
	&& ln -sf /proc/1/fd/2 /var/log/nginx/error.log

COPY ./docker/sysctl.conf

COPY nginx-ingress nginx/ingress.tmpl nginx/nginx.conf.tmpl /

RUN rm /etc/nginx/conf.d/*

CMD ["sysctl -p"]

ENTRYPOINT ["/nginx-ingress"]

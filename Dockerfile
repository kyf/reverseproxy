FROM busybox

ADD ./reverseproxy /bin/

EXPOSE 80

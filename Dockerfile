FROM        quay.io/prometheus/busybox:latest
MAINTAINER  Fernando Crespo Grávalos <fcgravalos@gmail.com>

COPY instaclustr_exporter /bin/instaclustr_exporter

EXPOSE     9279
ENTRYPOINT [ "/bin/instaclustr_exporter" ]

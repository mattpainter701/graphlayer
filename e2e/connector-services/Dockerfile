FROM debian:bookworm-slim
RUN apt-get update && apt-get install --yes --no-install-recommends openssh-server samba \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --create-home --shell /bin/bash netfixture \
    && mkdir -p /run/sshd /srv/configs/nested
# Synthetic fixture files must be readable by the service account even when
# the checkout was created under a private runner's restrictive umask.
COPY --chmod=0644 platform/firewall/testdata/enterprise.asa.txt /srv/configs/enterprise.asa.txt
COPY --chmod=0644 platform/firewall/testdata/asa_path_sample.txt /srv/configs/asa_path_sample.txt
COPY --chmod=0644 platform/firewall/testdata/panos_sample.txt /srv/configs/panos_sample.txt
COPY --chmod=0644 platform/firewall/testdata/fortigate_sample.txt /srv/configs/fortigate_sample.txt
COPY --chmod=0644 platform/firewall/testdata/enterprise.asa.txt /srv/configs/nested/duplicate.asa.txt
COPY e2e/connector-services/start.sh /usr/local/bin/start-connector-services
COPY e2e/connector-services/show.sh /usr/local/bin/show
RUN chmod 0755 /usr/local/bin/start-connector-services /usr/local/bin/show
EXPOSE 22 445
CMD ["/usr/local/bin/start-connector-services"]

FROM hashicorp/vault:1.16
RUN apk add --no-cache jq curl bash
COPY init.sh /init.sh
COPY policy.hcl /policy.hcl
RUN chmod +x /init.sh
ENTRYPOINT ["/init.sh"]

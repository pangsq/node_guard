FROM scratch

ADD node_guard /

ENTRYPOINT [ "/node_guard" ]
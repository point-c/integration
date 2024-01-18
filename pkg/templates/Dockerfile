FROM caddy:{{ .Caddy }}-builder AS builder

RUN xcaddy build {{ range .Mods }} \
    --with {{ . }}{{ end }}

FROM caddy:{{ .Caddy }}

COPY --from=builder /usr/bin/caddy /usr/bin/caddy
COPY Caddyfile /etc/caddy/Caddyfile
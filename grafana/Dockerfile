FROM grafana/grafana-oss:latest

USER root

# Grafana OSS has no supported white-label setting. Replace only the public
# brand assets so dashboard behavior and provisioning remain unchanged.
COPY grafana/branding/nms-logo.svg /usr/share/grafana/public/img/grafana_icon.svg
COPY grafana/branding/nms-logo.svg /usr/share/grafana/public/img/grafana_mask_icon.svg
COPY grafana/branding/fav32.png /usr/share/grafana/public/img/fav32.png
COPY grafana/branding/apple-touch-icon.png /usr/share/grafana/public/img/apple-touch-icon.png

USER grafana

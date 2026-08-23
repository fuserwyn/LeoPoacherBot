FROM grafana/grafana:latest

# Отключаем необходимость первого логина (для Railway нужна готовность к работе)
ENV GF_SECURITY_ADMIN_PASSWORD=admin
ENV GF_SECURITY_ADMIN_USER=admin
ENV GF_INSTALL_PLUGINS=""

# Копируем datasource конфиг Prometheus
COPY provisioning/datasources /etc/grafana/provisioning/datasources

# Копируем дашборды
COPY provisioning/dashboards /etc/grafana/provisioning/dashboards
COPY provisioning/dashboard-provider.yml /etc/grafana/provisioning/dashboards/dashboard-provider.yml

EXPOSE 3000

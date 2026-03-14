CLUSTER_NAME=bao-cluster
NETWORK=app-network

up:
	# Сеть и инфраструктура
	docker network create $(NETWORK) || true
	docker compose up -d

	# Создание кластера с Ingress (порт 80)
	# Флаг --host-alias позволяет k8s видеть Docker Compose по имени host.k3d.internal
	k3d cluster create $(CLUSTER_NAME) \
		--network $(NETWORK) \
		-p "80:80@loadbalancer" \
		--host-alias "$$(docker network inspect $(NETWORK) -f '{{range .IPAM.Config}}{{.Gateway}}{{end}}'):host.k3d.internal" \
		--k3s-arg "--disable=traefik@server:*"

	# Установка Nginx Ingress Controller (взамен стандартного Traefik)
	kubectl apply -f https://raw.githubusercontent.com

	# Metrics Server для работы HPA
	kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
	kubectl patch deployment metrics-server -n kube-system --type='json' \
		-p='[{"op": "add", "path": "/spec/template/spec/containers/0/args/-", "value": "--kubelet-insecure-tls"}]'

	# Сборка и деплой
	docker build -t bao-fight:local .
	k3d image import bao-fight:local -c $(CLUSTER_NAME)

	# Ждем готовности контроллера и деплоим
	sleep 20
	helm upgrade --install bao-fight ./deploy/helm/bao-fight --set image.tag=local --wait

down:
	k3d cluster delete $(CLUSTER_NAME)
	docker compose down

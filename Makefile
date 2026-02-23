docker-build:
	docker build -t go-proxy-lb .

docker-run:
	docker-compose up -d

docker-down:
	docker-compose down

docker-logs:
	docker-compose logs -f
	
docker-restart: docker-down docker-run

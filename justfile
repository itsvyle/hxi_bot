config-path := "./configs/esteban-test.docker-compose.yml"

default:
    just --list

start:
    docker compose -f {{config-path}} up -d --build
restart:
    docker compose -f {{config-path}} restart
logs:
    docker compose -f {{config-path}} logs -f
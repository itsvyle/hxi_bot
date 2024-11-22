config-path := "./configs/esteban-test.docker-compose.yml"

default:
    just --list

start:
    docker compose -f {{config-path}} up -d --build
stop:
    docker compose -f {{config-path}} down
restart:
    docker compose -f {{config-path}} restart
logs:
    docker compose -f {{config-path}} logs -f
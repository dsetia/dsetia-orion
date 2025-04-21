# securite

The 'lanuch.sh' script will pull all nginx, minio and postgres containers. Using docker compose
orchestrates the containers.

Dependencies
 - docker
   - Follow the link for details instruction on installing docker in Ubuntu https://docs.docker.com/engine/install/ubuntu/
 - docker-compose
   - sudo apt install docker-compose # on Ubuntu

Known Issues:
 - The 'launch.sh' script doesn't check for existence of the service on the host machine.  For
   example it doesn't check if 'postgres' is running and stop the starting of the container.

Commands:
docker-compose build
docker-compose up -d

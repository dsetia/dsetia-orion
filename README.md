# securite

The 'launch.sh' script will pull all nginx, minio and postgres containers. Using docker compose
orchestrates the containers.

Dependencies
 - docker
   - Follow the link for details instruction on installing docker in Ubuntu 
     - https://docs.docker.com/engine/install/ubuntu/
     - https://www.digitalocean.com/community/tutorials/how-to-install-and-use-docker-on-ubuntu-20-04
 - docker-compose
   - sudo apt install docker-compose # on Ubuntu
 - MinIO Client (mc)
   There are two options to use docker container or install locally
   - Docker Container
     - docker pull minio/mc
     - mc alias set myminio http://localhost:9000 minioadmin minioadmin
   - Install locally
     - curl https://dl.min.io/client/mc/release/linux-amd64/mc --create-dirs -o $HOME/minio-binaries/mc
     - chmod +x $HOME/minio-binaries/mc
     - export PATH=$PATH:$HOME/minio-binaries/
     - mc --help

Known Issues
 - The 'launch.sh' script doesn't check for existence of the service on the host machine.  For
   example it doesn't check if 'postgres' is running and stop the starting of the container.

Commands
 - docker-compose build
 - docker-compose up -d

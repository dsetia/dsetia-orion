RUN go mod init provisioner

# build provisioner
go get github.com/google/uuid github.com/lib/pq
go build -o provisioner main.go dbutil.go

# Ansible deployment of sensor
 ansible-playbook -i sensor_inventory deploy-sensor.yml

# SSH deployment
 ./deploy-sensor.sh sensor_hostname ../sensor-provision.tar.gz

Cloud Init
runcmd:
  - wget http://provisioner-host/sensor-provision.tar.gz -O /tmp/sensor-provision.tar.gz
  - tar -xzf /tmp/sensor-provision.tar.gz -C /tmp
  - cd /tmp/sensor-provision
  - chmod +x init-sensor.sh
  - ./init-sensor.sh

Following might be needed in init-sensor if go environment is needed.
Should not be needed if we are linking statically
# Install Go runtime (if not already installed)
# GO_VERSION="1.20"
# if ! command -v go &> /dev/null; then
#     echo "Installing Go $GO_VERSION..."
#     curl -LO https://golang.org/dl/go${GO_VERSION}.linux-amd64.tar.gz
#     tar -C /usr/local -xzf go${GO_VERSION}.linux-amd64.tar.gz
#     rm go${GO_VERSION}.linux-amd64.tar.gz
#     echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile
#     export PATH=$PATH:/usr/local/go/bin
# fi


# testing
docker cp sensor:/var/log/updater.log .
Files:
provisioner/Dockerfile.sensor: docker file for local testing
provisioner/config/updater-config-template.json: template file for sensor's updater-config.json
provisioner/config/provision-config.json: Override config file for above template (generates updater-config.json)
provisioner/entrypoint.sh: docker entry point for local testing; launches supervisord
provisioner/debug-entrypoint.sh: docker debug entry point for local testing; drops to shell
provisioner/deploy-sensor.sh: plaeholder for deployment of tarball to destination using scp/ssh
provisioner/deploy-sensor.yml: placeholder for deployment of tarbell to destination using ansible
provisioner/hello_world.sh: placeholder for actual suricata binary for testing etc.
provisioner/init-sensor.sh: runs on the sensor for initialization; part of the tarball
provisioner/main.go: Provisioner Go code
provisioner/supervisor/hndr.conf: initial suricata supervisord conf file
provisioner/supervisor/updater.conf: updater ssupervisord conf file

# test provisioner
# launch cloud service first using
# cd ..; ./launch provisioner/provisioner/docker-compose.override.yml
# this will create a new network and launch api server using testdb
# instead of pgdb
./test_provisioner.sh

# Build provisioner package and upload to minio bucket "provisioner"
cd provisioner
./deploy.sh provisioner ../config/minio_config.json

# build sensor package and upload to minio bucket "sensor"
# tenant ID is required
./deploy.sh sensor ../config/minio_config.json 1

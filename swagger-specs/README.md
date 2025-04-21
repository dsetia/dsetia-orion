# Viewing the Specification using Swagger

* Using docker container
    - docker pull swaggerapi/swagger-ui
    - docker run -itd --rm --name swagger -e SWAGGER_JSON=/swagger-specs/swupdater-apis.json -v /opt/tmp/work/or1/swagger-specs/swupdater-apis.json:/swagger-specs/swupdater-apis.json -p 9080:8080 swaggerapi/swagger-ui
    - http://localhost:9080

# woongkie-talkie
Simple Chat Program


## start redis using docker-compose
```
sudo docker-compose up --build -d
```

## start chat server
```
go run . serve
```

## start chat server using docker-compose
#### 1. Using Image Lightweight

- start up
```
sudo docker build . -f Dockerfile.builder -t builder

sudo docker build --build-arg=BUILDER_IMAGE=builder . -f Dockerfile -t woongkie-talkie

docker-compose up --build -d
```

- shut down
```
docker-compose down
```

#### 2. a simple way of running service
- start up
```
docker-compose -f docker-compose.integration.yml up --build -d
```

- shut down
```
docker-compose -f docker-compose.integration.yml down
```
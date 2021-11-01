# Running locally

## Initial Setup

```shell
yarn
make all
```

## frontend-service

`make all run`

## ui

To start the ui itself, run
```shell
(cd ../../pkg/api/; make all)
yarn && yarn start
```

## other

Note that you probably also need the `cd-service`.
`cd ../cd-service; WITHOUT_DOCKER=true make run`


# Deploy

Run this, but adapt the image name first for the project (here 'nemo')
```shell
make clean
yarn
make release
```

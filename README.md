# Link shortener

This project is a simple link shortener written for fun and practice. 

## Build image

```bash
docker build -t saphka/short-link .
```

## Running locally

```bash
docker compose up -d
make run
```

## Configuration

Env vars
```env
# App port, 8080
LINK_PORT 
# Server shutdown period, 5s
LINK_SHUTDOWN_PERIOD
# DB host, localhost
LINK_DATABASE_HOST
# DB port, 5432
LINK_DATABASE_PORT
# DB name, postgres
LINK_DATABASE_NAME
# DB user, postgres
LINK_DATABASE_USER
# DB password, postgres
LINK_DATABASE_PASSWORD
```
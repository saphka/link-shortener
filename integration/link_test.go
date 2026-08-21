//go:build integration

// nolint:errcheck,gochecknoglobals,noctx
package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/saphka/link-shortener/internal/app"
	"github.com/saphka/link-shortener/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go/modules/compose"
	"github.com/testcontainers/testcontainers-go/wait"
)

const localhost = "localhost"
const serverPort = 9090
const database = "postgres"
const databasePort = 5432

var stack *compose.DockerCompose
var serverUrl = fmt.Sprintf("http://%s:%d", localhost, serverPort)

var dbConn = fmt.Sprintf(
	"postgres://%s:%s@%s:%d/%s",
	database,
	database,
	localhost,
	databasePort,
	database,
)

func TestMain(m *testing.M) {
	var err error
	stack, err = compose.NewDockerCompose("../docker-compose.yaml")
	if err != nil {
		log.Fatalf("cannot create docker compose: %v", err)
	}

	err = stack.
		WaitForService("liquibase", wait.ForExit()).
		Up(
			context.Background(),
			compose.Wait(true),
		)
	if err != nil {
		log.Fatalf("cannot start docker compose: %v", err)
	}

	defer func() {
		err := stack.Down(
			context.Background(),
			compose.RemoveOrphans(true),
			compose.RemoveVolumes(true),
			compose.RemoveImagesLocal,
		)
		if err != nil {
			log.Printf("cannot stop docker compose: %v", err)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app, err := app.NewApp(ctx, "link_test", config.Config{
		Port:           serverPort,
		ShutdownPeriod: 200 * time.Microsecond,
		DB: config.Database{
			Host:     localhost,
			Port:     databasePort,
			Name:     database,
			User:     database,
			Password: database,
		},
	})
	if err != nil {
		log.Fatalf("cannot start app: %v", err)
	}

	go app.Run()

	exitCode := m.Run()
	cancel()
	os.Exit(exitCode)
}

func TestLinkCreate(t *testing.T) {
	resp, err := http.Post(
		serverUrl+"/link",
		"application/json",
		strings.NewReader(`{"url":"https://example.com"}`),
	)
	assert.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var respBody struct {
		Key string `json:"key"`
		Url string `json:"url"`
	}
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	assert.NoError(t, err)

	assert.Len(t, respBody.Key, 16)
	assert.Equal(t, "https://example.com", respBody.Url)

	conn, err := pgx.Connect(context.Background(), dbConn)
	assert.NoError(t, err)
	defer conn.Close(context.Background())

	var rowId int64
	err = conn.QueryRow(
		context.Background(),
		`SELECT id
			FROM shortlink
			WHERE short_key = $1
		`, respBody.Key,
	).Scan(&rowId)
	assert.NoError(t, err)
	assert.Positive(t, rowId)
}

func TestLinkRedirect(t *testing.T) {
	conn, err := pgx.Connect(context.Background(), dbConn)
	assert.NoError(t, err)
	defer conn.Close(context.Background())

	var rowId int64
	err = conn.QueryRow(
		context.Background(),
		`INSERT INTO shortlink
			(short_key, full_url)
			VALUES ($1, $2)
			RETURNING id
		`, "the0short0key", "http://example2.com",
	).Scan(&rowId)
	assert.NoError(t, err)
	assert.Positive(t, rowId)

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Get(serverUrl + "/l/the0short0key")
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusFound, resp.StatusCode)
	assert.Equal(t, "http://example2.com", resp.Header.Get("Location"))
}

func TestNoRedirect(t *testing.T) {
	conn, err := pgx.Connect(context.Background(), dbConn)
	assert.NoError(t, err)
	defer conn.Close(context.Background())

	var rowId int64
	err = conn.QueryRow(
		context.Background(),
		`INSERT INTO shortlink
			(short_key, full_url)
			VALUES ($1, $2)
			RETURNING id
		`, "the0other0key", "http://example3.com",
	).Scan(&rowId)
	assert.NoError(t, err)
	assert.Positive(t, rowId)

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Get(serverUrl + "/l/the0wrong0key")
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

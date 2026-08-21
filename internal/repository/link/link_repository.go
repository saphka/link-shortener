package link

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrLinkNotFound    = errors.New("link not found by short key")
	ErrLinkKeyConflict = errors.New("link short key conflit. try again")
)

type linkRepo struct {
	pool   *pgxpool.Pool
	keygen func() string
}

const keySize int = 16

func NewLinkRepo(pool *pgxpool.Pool) (*linkRepo, error) {
	return &linkRepo{
		pool:   pool,
		keygen: randomString,
	}, nil
}

func (r *linkRepo) CreateLink(ctx context.Context, url string) (ShortLink, error) {
	var result ShortLink

	return r.doInTxn(ctx, func(ctx context.Context, tx pgx.Tx) (ShortLink, error) {
		key := r.keygen()
		err := tx.QueryRow(ctx,
			`INSERT INTO shortlink(short_key, full_url) 
				VALUES ($1, $2) 
				ON CONFLICT DO NOTHING
				RETURNING id
			`, key, url,
		).Scan(&result.Id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return result, ErrLinkKeyConflict
			}
			return result, fmt.Errorf("cannot scan returned id: %w", err)
		}
		result.Key = key
		result.Url = url
		return result, nil
	})
}

func (r *linkRepo) GetLink(ctx context.Context, key string) (ShortLink, error) {
	return r.doInConnection(
		ctx,
		func(ctx context.Context, pooledConn *pgxpool.Conn) (ShortLink, error) {
			var result ShortLink
			err := pooledConn.QueryRow(ctx,
				`SELECT id, short_key, full_url 
					FROM shortlink 
					WHERE short_key = $1 
					LIMIT 1
				`, key,
			).Scan(&result.Id, &result.Key, &result.Url)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return result, ErrLinkNotFound
				}
				return result, fmt.Errorf("cannot scan returned link: %w", err)
			}
			return result, nil
		},
	)
}

func randomString() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, keySize)

	for i := range result {
		result[i] = charset[rand.IntN(len(charset))] //nolint:gosec
	}
	return string(result)
}

func (r *linkRepo) doInConnection(
	ctx context.Context,
	op func(ctx context.Context, pooledConn *pgxpool.Conn) (ShortLink, error),
) (ShortLink, error) {
	pooledConn, err := r.pool.Acquire(ctx)
	if err != nil {
		return ShortLink{}, fmt.Errorf("cannot accuire connection: %w", err)
	}
	defer pooledConn.Release()

	return op(ctx, pooledConn)
}

func (r *linkRepo) doInTxn(
	ctx context.Context,
	op func(ctx context.Context, tx pgx.Tx) (ShortLink, error),
) (ShortLink, error) {
	return r.doInConnection(
		ctx,
		func(ctx context.Context, pooledConn *pgxpool.Conn) (ShortLink, error) {
			var result ShortLink
			tx, err := pooledConn.Begin(ctx)
			if err != nil {
				return result, fmt.Errorf("cannot create tx: %w", err)
			}
			defer func() {
				err := tx.Rollback(ctx)
				if err != nil && !errors.Is(err, pgx.ErrTxClosed) {
					slog.Error("error returned when rolling back txn", slog.Any("error", err))
				}
			}()

			if result, err = op(ctx, tx); err != nil {
				return result, err
			}

			if err := tx.Commit(ctx); err != nil {
				return result, fmt.Errorf("cannot commit txn: %w", err)
			}
			return result, nil
		},
	)
}

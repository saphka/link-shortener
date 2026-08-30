package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/getkin/kin-openapi/openapi3"
	middleware "github.com/oapi-codegen/nethttp-middleware"
	"github.com/saphka/link-shortener/internal/repository/link"
)

//go:generate go tool oapi-codegen -config ../../oapi-codegen.yaml ../../api/openApi.yaml

var (
	errParseUrl          = errors.New("cannot parse url")
	errSchemeUnsupported = errors.New("unsupported url scheme")
	errUrlNoHost         = errors.New("link has no host")
)

type Server struct {
	repo LinkRepository
}

type LinkRepository interface {
	CreateLink(ctx context.Context, url string) (link.ShortLink, error)
	GetLink(ctx context.Context, key string) (link.ShortLink, error)
}

func NewServer(mux *http.ServeMux, repo LinkRepository) (http.Handler, error) {
	server := &Server{
		repo: repo,
	}

	mw, err := createMiddlewares()
	if err != nil {
		return nil, fmt.Errorf("cannot create middlewares: %w", err)
	}

	oapiMux := HandlerWithOptions(
		NewStrictHandler(server, []StrictMiddlewareFunc{}),
		StdHTTPServerOptions{
			BaseRouter:  mux,
			Middlewares: mw,
		},
	)

	return oapiMux, nil
}

// (POST /link).
func (s *Server) PostLink(
	ctx context.Context,
	request PostLinkRequestObject,
) (PostLinkResponseObject, error) {
	err := validateUrl(request.Body.Url)
	if err != nil {
		return PostLink400JSONResponse{
			Message: err.Error(),
		}, nil
	}

	shortLink, err := s.repo.CreateLink(ctx, request.Body.Url)
	if err != nil {
		if errors.Is(err, link.ErrLinkKeyConflict) {
			message := err.Error()
			return PostLink409JSONResponse{
				Message: message,
			}, nil
		}
		return nil, fmt.Errorf("cannot create short link. %w", err)
	}
	return PostLink201JSONResponse{Key: shortLink.Key, Url: shortLink.Url}, nil
}

// (GET /l/{short-key}).
func (s *Server) GetLShortKey(
	ctx context.Context,
	request GetLShortKeyRequestObject,
) (GetLShortKeyResponseObject, error) {
	shortLink, err := s.repo.GetLink(ctx, request.ShortKey)
	if err != nil {
		if errors.Is(err, link.ErrLinkNotFound) {
			return GetLShortKey404JSONResponse{
				Message: err.Error(),
			}, nil
		}
		return nil, err
	}
	err = validateUrl(shortLink.Url)
	if err != nil {
		return GetLShortKey400JSONResponse{
			Message: err.Error(),
		}, nil
	}
	return GetLShortKey302Response{
		Headers: GetLShortKey302ResponseHeaders{
			Location: shortLink.Url,
		},
	}, nil
}

func createMiddlewares() ([]MiddlewareFunc, error) {
	result := make([]MiddlewareFunc, 0, 1)
	rawSpecData, err := rawSpec()
	if err != nil {
		return result, err
	}
	spec, err := openapi3.NewLoader().LoadFromData(rawSpecData)
	if err != nil {
		return result, fmt.Errorf("cannot load spec: %w", err)
	}

	result = append(result, middleware.OapiRequestValidatorWithOptions(spec, &middleware.Options{
		ErrorHandler: func(w http.ResponseWriter, message string, statusCode int) {
			errorBody := ErrorResponse{
				Message: message,
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(statusCode)
			if err := json.NewEncoder(w).Encode(errorBody); err != nil {
				slog.Error("cannot write error body", slog.Any("error", err))
			}
		},
	}))

	return result, nil
}

func validateUrl(rawUrl string) error {
	parsedUrl, err := url.Parse(rawUrl)
	if err != nil {
		return fmt.Errorf("%w: %w", errParseUrl, err)
	}
	if parsedUrl.Scheme != "http" && parsedUrl.Scheme != "https" {
		return fmt.Errorf("%w: %s", errSchemeUnsupported, parsedUrl.Scheme)
	}
	if parsedUrl.Host == "" {
		return errUrlNoHost
	}
	return nil
}

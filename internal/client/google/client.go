package google

import (
	"fmt"
	"io"
	"net/http"

	"golang.org/x/oauth2"

	"github.com/rs/zerolog"
)

type Google struct {
	logger zerolog.Logger
}

// Получаем данные о пользователе из Google
func (g *Google) GetUserInfo(token *oauth2.Token) ([]byte, error) {
	const url = "https://www.googleapis.com/oauth2/v2/userinfo?access_token="

	response, err := http.Get(url + token.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed getting user info: %s", err.Error())
	}

	defer func() {
		if err = response.Body.Close(); err != nil {
			g.logger.Error().Err(err).Msg("failed to close body")
		}
	}()

	contents, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed reading response body: %s", err.Error())
	}

	return contents, nil
}

func New(logger zerolog.Logger) *Google {
	return &Google{
		logger: logger,
	}
}

package upgrade

import (
	"context"
	"errors"
)

func downloadWithTUF(_ context.Context, _ string) (string, error) {
	return "", errors.New("tuf download is not implemented yet")
}

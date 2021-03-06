package indexer

import (
	"context"
	"io/ioutil"
	"os"

	"github.com/pkg/errors"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/db"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/tar"
)

func fetchRepository(ctx context.Context, db db.DB, gitserverClient gitserver.Client, repositoryID int, commit string) (string, error) {
	tempDir, err := ioutil.TempDir("", "")
	if err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(tempDir)
		}
	}()

	archive, err := gitserverClient.Archive(ctx, db, repositoryID, commit)
	if err != nil {
		return "", errors.Wrap(err, "gitserver.Archive")
	}

	if err := tar.Extract(tempDir, archive); err != nil {
		return "", errors.Wrap(err, "tar.Extract")
	}

	return tempDir, nil
}

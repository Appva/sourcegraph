package repos

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/pkg/errors"
	"github.com/sourcegraph/go-diff/diff"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/types"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/vcs/git"
)

// computeGitDiffStat returns the raw output from git diff for the given repo, base,
// and head OID, ready to be fed to diff.NewMultiFileDiffReader().
func computeGitDiffStat(ctx context.Context, repo *Repo, baseRefOid, headRefOid string) (*diff.Stat, error) {
	// Let's translate our local Repo struct into something we can give to the
	// gitserver.
	//
	// TODO: figure out why this doesn't always resolve on startup, even when
	// using the non-quick version.
	rr, err := backend.GitRepo(ctx, &types.Repo{
		Name:         api.RepoName(repo.Name),
		ExternalRepo: repo.ExternalRepo,
	})
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("getting backend repo for repo: %v", repo))
	}

	// This is mostly cargo culted from
	// graphqlbackend.computeRepositoryComparisonDiff(), removing the checks
	// that don't apply to the campaign case (by definition, we always have a
	// base and head OID), and the pagination code that isn't relevant here.
	rangeSpec := baseRefOid + "..." + headRefOid
	if strings.HasPrefix(rangeSpec, "-") || strings.HasPrefix(rangeSpec, ".") {
		return nil, fmt.Errorf("invalid diff range argument: %q", rangeSpec)
	}

	// We can't use --stat (or --numstat) here because we have to calculate the
	// changed lines, which git doesn't support: instead, we have to use go-diff
	// to get the semantics expected by Sourcegraph users. So let's grab the
	// full diff and use go-diff to calculate the stat.
	rdr, err := git.ExecReader(ctx, rr, []string{
		"diff",
		"--find-renames",
		// TODO(eseliger): Enable once we have support for copy detection in go-diff
		// and actually expose a `isCopy` field in the api, otherwise this
		// information is thrown away anyways.
		// "--find-copies",
		"--full-index",
		"--inter-hunk-context=3",
		"--no-prefix",
		rangeSpec,
		"--",
	})
	if err != nil {
		return nil, errors.Wrap(err, "executing git diff")
	}
	defer rdr.Close()

	dr := diff.NewMultiFileDiffReader(rdr)
	stat := diff.Stat{}
	for {
		fileDiff, err := dr.ReadFile()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, errors.Wrap(err, "getting file diff")
		}

		fileStat := fileDiff.Stat()
		stat.Added += fileStat.Added
		stat.Changed += fileStat.Changed
		stat.Deleted += fileStat.Deleted
	}

	return &stat, nil
}

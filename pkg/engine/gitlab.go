package engine

import (
	"fmt"
	"runtime"

	gogit "github.com/go-git/go-git/v5"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/trufflesecurity/trufflehog/v3/pkg/context"
	"github.com/trufflesecurity/trufflehog/v3/pkg/pb/sourcespb"
	"github.com/trufflesecurity/trufflehog/v3/pkg/sources"
	"github.com/trufflesecurity/trufflehog/v3/pkg/sources/git"
	"github.com/trufflesecurity/trufflehog/v3/pkg/sources/gitlab"
)

// ScanGitLab scans GitLab with the provided configuration.
func (e *Engine) ScanGitLab(ctx context.Context, c sources.GitlabConfig) error {
	logOptions := &gogit.LogOptions{}
	opts := []git.ScanOption{
		git.ScanOptionFilter(c.Filter),
		git.ScanOptionLogOptions(logOptions),
	}
	scanOptions := git.NewScanOptions(opts...)

	connection := &sourcespb.GitLab{}

	switch {
	case len(c.Token) > 0:
		connection.Credential = &sourcespb.GitLab_Token{
			Token: c.Token,
		}
	default:
		return fmt.Errorf("must provide token")
	}

	if len(c.Endpoint) > 0 {
		connection.Endpoint = c.Endpoint
	}

	if len(c.Repos) > 0 {
		connection.Repositories = c.Repos
	}

	var conn anypb.Any
	err := anypb.MarshalFrom(&conn, connection, proto.MarshalOptions{})
	if err != nil {
		ctx.Logger().Error(err, "failed to marshal gitlab connection")
		return err
	}

	handle, err := e.sourceManager.Enroll(ctx, "trufflehog - gitlab", new(gitlab.Source).Type(),
		func(ctx context.Context, jobID, sourceID int64) (sources.Source, error) {
			gitlabSource := gitlab.Source{}
			if err := gitlabSource.Init(ctx, "trufflehog - gitlab", jobID, sourceID, true, &conn, runtime.NumCPU()); err != nil {
				return nil, err
			}
			gitlabSource.WithScanOptions(scanOptions)
			return &gitlabSource, nil
		})
	if err != nil {
		return err
	}
	_, err = e.sourceManager.ScheduleRun(ctx, handle)
	return err
}

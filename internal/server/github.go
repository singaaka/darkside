package server

import (
	"context"
	"database/sql"
	"errors"

	"connectrpc.com/connect"

	darksidev1 "github.com/singaaka/darkside/gen/go/darkside/v1"
	"github.com/singaaka/darkside/internal/db/dbgen"
	"github.com/singaaka/darkside/internal/github"
	"github.com/singaaka/darkside/internal/store"
)

type githubHandler struct {
	store *store.Store
}

func (h *githubHandler) GetStatus(ctx context.Context, _ *connect.Request[darksidev1.GetGitHubStatusRequest]) (*connect.Response[darksidev1.GetGitHubStatusResponse], error) {
	app, err := h.store.GetGitHubApp(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return connect.NewResponse(&darksidev1.GetGitHubStatusResponse{Connected: false}), nil
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&darksidev1.GetGitHubStatusResponse{
		Connected:  true,
		AppId:      app.AppID,
		AppSlug:    app.Slug,
		AppName:    app.Name,
		HtmlUrl:    app.HtmlUrl,
		InstallUrl: app.HtmlUrl + "/installations/new",
	}), nil
}

func (h *githubHandler) ListInstallations(ctx context.Context, _ *connect.Request[darksidev1.ListInstallationsRequest]) (*connect.Response[darksidev1.ListInstallationsResponse], error) {
	app, err := h.store.GetGitHubApp(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("github app not connected"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	client := github.NewApp(app.AppID, app.PrivateKey)
	live, err := client.ListInstallations(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}

	// Keep our local cache in sync so the webhook handler in Phase 4 can look
	// up installations without hitting GitHub on every push.
	out := make([]*darksidev1.Installation, 0, len(live))
	for _, ins := range live {
		_ = h.store.UpsertInstallation(ctx, dbgen.UpsertInstallationParams{
			ID:           ins.ID,
			AccountLogin: ins.Account.Login,
			AccountType:  ins.Account.Type,
		})
		out = append(out, &darksidev1.Installation{
			Id:           ins.ID,
			AccountLogin: ins.Account.Login,
			AccountType:  ins.Account.Type,
		})
	}
	return connect.NewResponse(&darksidev1.ListInstallationsResponse{Installations: out}), nil
}

func (h *githubHandler) ListInstallationRepos(ctx context.Context, req *connect.Request[darksidev1.ListInstallationReposRequest]) (*connect.Response[darksidev1.ListInstallationReposResponse], error) {
	app, err := h.store.GetGitHubApp(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("github app not connected"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	client := github.NewApp(app.AppID, app.PrivateKey)
	repos, err := client.ListInstallationRepos(ctx, req.Msg.InstallationId)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	out := make([]*darksidev1.Repo, 0, len(repos))
	for _, r := range repos {
		out = append(out, &darksidev1.Repo{
			Id:            r.ID,
			FullName:      r.FullName,
			DefaultBranch: r.DefaultBranch,
			Private:       r.Private,
			Description:   r.Description,
		})
	}
	return connect.NewResponse(&darksidev1.ListInstallationReposResponse{Repos: out}), nil
}

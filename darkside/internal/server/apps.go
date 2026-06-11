package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	darksidev1 "github.com/singaaka/darkside/gen/go/darkside/v1"
	"github.com/singaaka/darkside/internal/ageenv"
	"github.com/singaaka/darkside/internal/config"
	"github.com/singaaka/darkside/internal/db/dbgen"
	"github.com/singaaka/darkside/internal/manifest"
	"github.com/singaaka/darkside/internal/store"
)

type appsHandler struct {
	store *store.Store
	cfg   *config.Config
}

var appNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,38}[a-z0-9]$`)

func (h *appsHandler) List(ctx context.Context, _ *connect.Request[darksidev1.ListAppsRequest]) (*connect.Response[darksidev1.ListAppsResponse], error) {
	apps, err := h.store.ListApps(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*darksidev1.App, 0, len(apps))
	for _, a := range apps {
		out = append(out, appToProto(a))
	}
	return connect.NewResponse(&darksidev1.ListAppsResponse{Apps: out}), nil
}

func (h *appsHandler) Get(ctx context.Context, req *connect.Request[darksidev1.GetAppRequest]) (*connect.Response[darksidev1.App], error) {
	a, err := h.store.GetApp(ctx, req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	return connect.NewResponse(appToProto(a)), nil
}

// Create via ConnectRPC is kept for backwards compat but the full-featured
// create (with age key generation) lives at POST /api/v1/apps.
func (h *appsHandler) Create(ctx context.Context, req *connect.Request[darksidev1.CreateAppRequest]) (*connect.Response[darksidev1.App], error) {
	name := req.Msg.Name
	if !appNameRe.MatchString(name) {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("name must match [a-z0-9][a-z0-9-]{1,38}[a-z0-9] (got %q)", name))
	}
	if req.Msg.RepoFullName == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("repo_full_name is required"))
	}
	if req.Msg.InstallationId == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("installation_id is required"))
	}
	kp, err := ageenv.GenerateKeypair()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	id := uuid.NewString()
	if err := h.store.CreateApp(ctx, dbgen.CreateAppParams{
		ID:             id,
		Name:           name,
		RepoFullName:   req.Msg.RepoFullName,
		InstallationID: req.Msg.InstallationId,
		Branch:         "main",
		EnvFile:        "env.age",
		AgePublicKey:   kp.PublicKey,
		AgePrivateKey:  kp.PrivateKey,
		AgeKeyID:       "key-v1",
	}); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	a, err := h.store.GetApp(ctx, id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(appToProto(a)), nil
}

func (h *appsHandler) GetManifestSample(ctx context.Context, req *connect.Request[darksidev1.GetManifestSampleRequest]) (*connect.Response[darksidev1.GetManifestSampleResponse], error) {
	a, err := h.store.GetApp(ctx, req.Msg.AppId)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("app not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&darksidev1.GetManifestSampleResponse{
		Toml: manifest.Sample(a.Name, a.Branch, a.EnvFile, a.AgeKeyID, a.AgePublicKey),
	}), nil
}

func appToProto(a dbgen.App) *darksidev1.App {
	return &darksidev1.App{
		Id:             a.ID,
		Name:           a.Name,
		RepoFullName:   a.RepoFullName,
		InstallationId: a.InstallationID,
		CreatedAtUnix:  a.CreatedAt.Unix(),
	}
}

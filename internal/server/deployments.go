package server

import (
	"context"
	"database/sql"
	"errors"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	darksidev1 "github.com/singaaka/darkside/gen/go/darkside/v1"
	"github.com/singaaka/darkside/internal/builder"
	"github.com/singaaka/darkside/internal/buildqueue"
	"github.com/singaaka/darkside/internal/db/dbgen"
	"github.com/singaaka/darkside/internal/github"
	"github.com/singaaka/darkside/internal/loghub"
	"github.com/singaaka/darkside/internal/store"
)

type deploymentsHandler struct {
	store    *store.Store
	hub      *loghub.Hub
	queue    *buildqueue.Queue
	makeJob  func(j builder.Job, gh *github.App, installID int64) buildqueue.Job
}

func (h *deploymentsHandler) List(ctx context.Context, req *connect.Request[darksidev1.ListDeploymentsRequest]) (*connect.Response[darksidev1.ListDeploymentsResponse], error) {
	limit := req.Msg.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := h.store.ListDeployments(ctx, dbgen.ListDeploymentsParams{
		AppID: req.Msg.AppId,
		Limit: int64(limit),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*darksidev1.Deployment, 0, len(rows))
	for _, d := range rows {
		out = append(out, h.deploymentToProto(ctx, d))
	}
	return connect.NewResponse(&darksidev1.ListDeploymentsResponse{Deployments: out}), nil
}

func (h *deploymentsHandler) Get(ctx context.Context, req *connect.Request[darksidev1.GetDeploymentRequest]) (*connect.Response[darksidev1.Deployment], error) {
	d, err := h.store.GetDeployment(ctx, req.Msg.Id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(h.deploymentToProto(ctx, d)), nil
}

func (h *deploymentsHandler) StreamLogs(
	ctx context.Context,
	req *connect.Request[darksidev1.StreamLogsRequest],
	stream *connect.ServerStream[darksidev1.StreamLogsResponse],
) error {
	phase := req.Msg.Phase
	if phase == "" {
		phase = "build"
	}

	sub, closed := h.hub.Subscribe(req.Msg.DeploymentId, phase)
	if closed {
		// Phase already finished — serve the persisted log in one chunk.
		log, err := h.store.GetDeploymentLog(ctx, dbgen.GetDeploymentLogParams{
			DeploymentID: req.Msg.DeploymentId,
			Phase:        phase,
		})
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		return stream.Send(&darksidev1.StreamLogsResponse{Chunk: log.Content})
	}
	defer sub.Close()

	for {
		select {
		case <-ctx.Done():
			return nil
		case line, ok := <-sub.Channel():
			if !ok {
				return nil
			}
			if err := stream.Send(&darksidev1.StreamLogsResponse{Chunk: line}); err != nil {
				return err
			}
		}
	}
}

func (h *deploymentsHandler) Redeploy(ctx context.Context, req *connect.Request[darksidev1.RedeployRequest]) (*connect.Response[darksidev1.Deployment], error) {
	src, err := h.store.GetDeployment(ctx, req.Msg.DeploymentId)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("source deployment not found"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if src.EnvironmentID == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			errors.New("source deployment has no environment — nothing to redeploy"))
	}
	env, err := h.store.GetEnvironmentByID(ctx, *src.EnvironmentID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	app, err := h.store.GetApp(ctx, src.AppID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	gh, err := h.store.GetGitHubApp(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			errors.New("github app not connected"))
	}

	depID := uuid.NewString()
	envID := env.ID
	if err := h.store.CreateDeployment(ctx, dbgen.CreateDeploymentParams{
		ID:            depID,
		AppID:         app.ID,
		EnvironmentID: &envID,
		CommitSha:     src.CommitSha,
		CommitMessage: "redeploy of " + src.CommitSha[:7] + " — " + src.CommitMessage,
		Status:        "pending",
	}); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	job := h.makeJob(builder.Job{
		DeploymentID: depID,
		AppID:        app.ID,
		AppName:      app.Name,
		RepoFullName: app.RepoFullName,
		Branch:       "",            // bypass branch lookup
		EnvOverride:  env.Name,      // force this env
		CommitSHA:    src.CommitSha,
		CommitMsg:    src.CommitMessage,
	}, github.NewApp(gh.AppID, gh.PrivateKey), app.InstallationID)

	if !h.queue.Submit(job) {
		_ = h.store.UpdateDeploymentStatus(ctx, dbgen.UpdateDeploymentStatusParams{
			ID:     depID,
			Status: "failed",
			Error:  strPtrLocal("build queue full"),
		})
		return nil, connect.NewError(connect.CodeResourceExhausted, errors.New("build queue full"))
	}

	d, err := h.store.GetDeployment(ctx, depID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(h.deploymentToProto(ctx, d)), nil
}

func strPtrLocal(s string) *string { return &s }

func (h *deploymentsHandler) deploymentToProto(ctx context.Context, d dbgen.Deployment) *darksidev1.Deployment {
	out := &darksidev1.Deployment{
		Id:             d.ID,
		AppId:          d.AppID,
		CommitSha:      d.CommitSha,
		CommitMessage:  d.CommitMessage,
		Status:         d.Status,
		StartedAtUnix:  d.StartedAt.Unix(),
	}
	if d.EnvironmentID != nil {
		out.EnvironmentId = *d.EnvironmentID
		// Fetch env name for the UI. Single query per deployment — fine at
		// list sizes we care about (50ish). Optimise later if it bites.
		if env, err := h.store.GetEnvironmentByID(ctx, *d.EnvironmentID); err == nil {
			out.EnvironmentName = env.Name
		}
	}
	if d.ImageTag != nil {
		out.ImageTag = *d.ImageTag
	}
	if d.NomadJobHcl != nil {
		out.NomadJobHcl = *d.NomadJobHcl
	}
	if d.EnvSnapshot != nil {
		out.EnvSnapshot = *d.EnvSnapshot
	}
	if d.Error != nil {
		out.Error = *d.Error
	}
	if d.FinishedAt != nil {
		out.FinishedAtUnix = d.FinishedAt.Unix()
	}
	return out
}

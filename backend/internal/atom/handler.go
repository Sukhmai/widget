// Package atom implements the AtomService connect handler: creating atoms from
// a URL and/or text, and reading them back with their resource metadata joined.
package atom

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
	"github.com/sukhmai/widget/backend/internal/db"
	atomv1 "github.com/sukhmai/widget/backend/internal/gen/atom/v1"
	"github.com/sukhmai/widget/backend/internal/gen/atom/v1/atomv1connect"
	"github.com/sukhmai/widget/backend/internal/push"
	"github.com/sukhmai/widget/backend/internal/resource"
)

const defaultPageSize = 20

// Handler serves the AtomService RPCs.
type Handler struct {
	db  *db.Queries
	rdb *redis.Client
}

var _ atomv1connect.AtomServiceHandler = (*Handler)(nil)

// New builds an AtomService handler over the given queries and redis client.
func New(queries *db.Queries, rdb *redis.Client) *Handler {
	return &Handler{db: queries, rdb: rdb}
}

func (h *Handler) CreateAtom(ctx context.Context, req *connect.Request[atomv1.CreateAtomRequest]) (*connect.Response[atomv1.CreateAtomResponse], error) {
	userID, err := parseUUID(req.Msg.UserId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid user_id"))
	}

	hasText := req.Msg.Text != ""
	hasURL := req.Msg.ResourceUrl != ""
	if !hasText && !hasURL {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("atom must have text or a resource_url"))
	}

	var resourceID pgtype.UUID
	if hasURL {
		r, err := h.db.UpsertResource(ctx, db.UpsertResourceParams{
			Url:    req.Msg.ResourceUrl,
			Source: string(resource.DetectSource(req.Msg.ResourceUrl)),
		})
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		resourceID = r.ID
		// Non-blocking: enqueue metadata fetch for the worker. A failure here
		// leaves the atom created but the resource unfetched; the worker will
		// pick it up on a later enqueue.
		_ = resource.Enqueue(ctx, h.rdb, r.ID.String())
	}

	atom, err := h.db.CreateAtom(ctx, db.CreateAtomParams{
		UserID:     userID,
		Text:       pgtype.Text{String: req.Msg.Text, Valid: hasText},
		ResourceID: resourceID,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	row, err := h.db.GetAtom(ctx, atom.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Non-blocking: notify subscribers so their widgets can refresh right
	// away. Best-effort — the widget's own poll interval is the fallback.
	_ = push.Enqueue(ctx, h.rdb, req.Msg.UserId)

	return connect.NewResponse(&atomv1.CreateAtomResponse{
		Atom: getAtomRowToProto(row),
	}), nil
}

func (h *Handler) GetAtom(ctx context.Context, req *connect.Request[atomv1.GetAtomRequest]) (*connect.Response[atomv1.GetAtomResponse], error) {
	id, err := parseUUID(req.Msg.AtomId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid atom_id"))
	}

	row, err := h.db.GetAtom(ctx, id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("atom not found"))
	}

	return connect.NewResponse(&atomv1.GetAtomResponse{
		Atom: getAtomRowToProto(row),
	}), nil
}

// GetLatestAtomFromUser returns the given user's most recent atom, or an empty
// response (Atom unset) when they have none. This backs the widget's display of
// a friend's current daily atom.
func (h *Handler) GetLatestAtomFromUser(ctx context.Context, req *connect.Request[atomv1.GetLatestAtomFromUserRequest]) (*connect.Response[atomv1.GetLatestAtomFromUserResponse], error) {
	userID, err := parseUUID(req.Msg.UserId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid user_id"))
	}

	row, err := h.db.GetLatestAtomFromUser(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return connect.NewResponse(&atomv1.GetLatestAtomFromUserResponse{}), nil
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&atomv1.GetLatestAtomFromUserResponse{
		Atom: latestAtomRowToProto(row),
	}), nil
}

// GetActiveAtomsFromUser returns the given user's atoms from the last 24
// hours, newest first. Backs the widget's per-friend story cycling.
func (h *Handler) GetActiveAtomsFromUser(ctx context.Context, req *connect.Request[atomv1.GetActiveAtomsFromUserRequest]) (*connect.Response[atomv1.GetActiveAtomsFromUserResponse], error) {
	userID, err := parseUUID(req.Msg.UserId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid user_id"))
	}

	rows, err := h.db.GetActiveAtomsFromUser(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	atoms := make([]*atomv1.Atom, len(rows))
	for i, row := range rows {
		atoms[i] = activeAtomsFromUserRowToProto(row)
	}

	return connect.NewResponse(&atomv1.GetActiveAtomsFromUserResponse{Atoms: atoms}), nil
}

// GetActiveFeedForSubscriber returns atoms from the last 24 hours across all
// of subscriber_id's subscriptions, merged newest first. Backs the widget's
// merged "All" story mode.
func (h *Handler) GetActiveFeedForSubscriber(ctx context.Context, req *connect.Request[atomv1.GetActiveFeedForSubscriberRequest]) (*connect.Response[atomv1.GetActiveFeedForSubscriberResponse], error) {
	subscriberID, err := parseUUID(req.Msg.SubscriberId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid subscriber_id"))
	}

	rows, err := h.db.GetActiveFeedForSubscriber(ctx, subscriberID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	atoms := make([]*atomv1.Atom, len(rows))
	for i, row := range rows {
		atoms[i] = activeFeedForSubscriberRowToProto(row)
	}

	return connect.NewResponse(&atomv1.GetActiveFeedForSubscriberResponse{Atoms: atoms}), nil
}

func (h *Handler) ListAtomsByUser(ctx context.Context, req *connect.Request[atomv1.ListAtomsByUserRequest]) (*connect.Response[atomv1.ListAtomsByUserResponse], error) {
	userID, err := parseUUID(req.Msg.UserId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid user_id"))
	}

	pageSize := req.Msg.PageSize
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}

	params := db.ListAtomsByUserParams{
		UserID:   userID,
		PageSize: pageSize + 1, // fetch one extra to determine has_next_page
	}

	if req.Msg.CursorTime != "" && req.Msg.CursorId != "" {
		t, err := time.Parse(time.RFC3339, req.Msg.CursorTime)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid cursor_time"))
		}
		params.CursorTime = pgtype.Timestamptz{Time: t, Valid: true}

		cursorID, err := parseUUID(req.Msg.CursorId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid cursor_id"))
		}
		params.CursorID = cursorID
	}

	rows, err := h.db.ListAtomsByUser(ctx, params)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	hasNextPage := len(rows) > int(pageSize)
	if hasNextPage {
		rows = rows[:pageSize]
	}

	atoms := make([]*atomv1.Atom, len(rows))
	for i, row := range rows {
		atoms[i] = listAtomRowToProto(row)
	}

	return connect.NewResponse(&atomv1.ListAtomsByUserResponse{
		Atoms:       atoms,
		HasNextPage: hasNextPage,
	}), nil
}

func (h *Handler) UpdateAtom(ctx context.Context, req *connect.Request[atomv1.UpdateAtomRequest]) (*connect.Response[atomv1.UpdateAtomResponse], error) {
	atomID, err := parseUUID(req.Msg.AtomId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid atom_id"))
	}

	hasText := req.Msg.Text != ""
	if err := h.db.UpdateAtomText(ctx, db.UpdateAtomTextParams{
		ID:   atomID,
		Text: pgtype.Text{String: req.Msg.Text, Valid: hasText},
	}); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	row, err := h.db.GetAtom(ctx, atomID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&atomv1.UpdateAtomResponse{
		Atom: getAtomRowToProto(row),
	}), nil
}

func (h *Handler) DeleteAtom(ctx context.Context, req *connect.Request[atomv1.DeleteAtomRequest]) (*connect.Response[atomv1.DeleteAtomResponse], error) {
	atomID, err := parseUUID(req.Msg.AtomId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid atom_id"))
	}

	if err := h.db.DeleteAtom(ctx, atomID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&atomv1.DeleteAtomResponse{}), nil
}

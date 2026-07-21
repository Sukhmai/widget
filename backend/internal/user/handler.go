// Package user implements the UserService connect handler: registering a
// device's discovery key (phone_hash) and resolving contacts to device UUIDs.
// It never stores or returns phone numbers in the clear — only opaque hashes
// supplied by the client — and keeps write identity (device_id) separate from
// the discovery key.
package user

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sukhmai/widget/backend/internal/db"
	userv1 "github.com/sukhmai/widget/backend/internal/gen/user/v1"
	"github.com/sukhmai/widget/backend/internal/gen/user/v1/userv1connect"
)

// Handler serves the UserService RPCs.
type Handler struct {
	db *db.Queries
}

var _ userv1connect.UserServiceHandler = (*Handler)(nil)

// New builds a UserService handler over the given queries.
func New(queries *db.Queries) *Handler {
	return &Handler{db: queries}
}

// parseUUID parses a canonical UUID string into a pgtype.UUID.
func parseUUID(s string) (pgtype.UUID, error) {
	var u pgtype.UUID
	if s == "" {
		return u, errors.New("empty uuid")
	}
	if err := u.Scan(s); err != nil {
		return u, err
	}
	return u, nil
}

func (h *Handler) RegisterUser(ctx context.Context, req *connect.Request[userv1.RegisterUserRequest]) (*connect.Response[userv1.RegisterUserResponse], error) {
	deviceID, err := parseUUID(req.Msg.DeviceId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid device_id"))
	}
	if len(req.Msg.PhoneHash) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("phone_hash is required"))
	}

	if _, err := h.db.UpsertUser(ctx, db.UpsertUserParams{
		DeviceID:  deviceID,
		PhoneHash: req.Msg.PhoneHash,
	}); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&userv1.RegisterUserResponse{}), nil
}

func (h *Handler) MatchContacts(ctx context.Context, req *connect.Request[userv1.MatchContactsRequest]) (*connect.Response[userv1.MatchContactsResponse], error) {
	if len(req.Msg.PhoneHashes) == 0 {
		return connect.NewResponse(&userv1.MatchContactsResponse{}), nil
	}

	rows, err := h.db.MatchContacts(ctx, req.Msg.PhoneHashes)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	matches := make([]*userv1.Match, len(rows))
	for i, row := range rows {
		matches[i] = &userv1.Match{
			PhoneHash: row.PhoneHash,
			DeviceId:  row.DeviceID.String(),
		}
	}

	return connect.NewResponse(&userv1.MatchContactsResponse{Matches: matches}), nil
}

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

// MatchContacts resolves which of the caller's contacts have registered. When
// the caller supplies its own device_id (subscriber_id), each match also
// implicitly subscribes it to that friend's atoms — there is no separate
// follow/unfollow step. An empty subscriber_id (e.g. an older client) simply
// skips subscribing, so this stays backwards compatible.
func (h *Handler) MatchContacts(ctx context.Context, req *connect.Request[userv1.MatchContactsRequest]) (*connect.Response[userv1.MatchContactsResponse], error) {
	if len(req.Msg.PhoneHashes) == 0 {
		return connect.NewResponse(&userv1.MatchContactsResponse{}), nil
	}

	rows, err := h.db.MatchContacts(ctx, req.Msg.PhoneHashes)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var subscriberID pgtype.UUID
	if req.Msg.SubscriberId != "" {
		subscriberID, err = parseUUID(req.Msg.SubscriberId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid subscriber_id"))
		}
	}

	matches := make([]*userv1.Match, len(rows))
	for i, row := range rows {
		matches[i] = &userv1.Match{
			PhoneHash: row.PhoneHash,
			DeviceId:  row.DeviceID.String(),
		}

		if subscriberID.Valid && row.DeviceID != subscriberID {
			if err := h.db.UpsertSubscription(ctx, db.UpsertSubscriptionParams{
				SubscriberID: subscriberID,
				TargetID:     row.DeviceID,
			}); err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		}
	}

	return connect.NewResponse(&userv1.MatchContactsResponse{Matches: matches}), nil
}

// RegisterPushToken stores this device's current APNs token, so a friend's
// post can trigger a background push telling this device to refresh its
// widget instead of relying solely on the timeline's own poll interval.
func (h *Handler) RegisterPushToken(ctx context.Context, req *connect.Request[userv1.RegisterPushTokenRequest]) (*connect.Response[userv1.RegisterPushTokenResponse], error) {
	deviceID, err := parseUUID(req.Msg.DeviceId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid device_id"))
	}
	if req.Msg.PushToken == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("push_token is required"))
	}

	if err := h.db.SetPushToken(ctx, db.SetPushTokenParams{
		DeviceID:  deviceID,
		PushToken: pgtype.Text{String: req.Msg.PushToken, Valid: true},
	}); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&userv1.RegisterPushTokenResponse{}), nil
}

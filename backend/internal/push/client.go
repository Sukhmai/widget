package push

import (
	"fmt"

	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/token"
)

// Client sends silent "refresh your widget" pushes via APNs.
type Client struct {
	apns  *apns2.Client
	topic string
}

// NewClient builds a Client authenticated with a .p8 APNs auth key. topic
// should be the containing app's bundle ID (not the widget extension's) —
// only the app can register for and receive remote notifications; it then
// tells WidgetCenter to reload the widget's timeline.
func NewClient(authKeyPath, keyID, teamID, topic string, production bool) (*Client, error) {
	authKey, err := token.AuthKeyFromFile(authKeyPath)
	if err != nil {
		return nil, fmt.Errorf("push: load auth key: %w", err)
	}

	tok := &token.Token{AuthKey: authKey, KeyID: keyID, TeamID: teamID}
	c := apns2.NewTokenClient(tok)
	if production {
		c = c.Production()
	} else {
		c = c.Development()
	}

	return &Client{apns: c, topic: topic}, nil
}

// SendSilent sends a content-available background push, telling the app to
// wake up and refresh its widgets. Delivery is best-effort — the OS may
// throttle or drop it — so the widget's own poll interval stays as a fallback.
func (c *Client) SendSilent(deviceToken string) error {
	res, err := c.apns.Push(&apns2.Notification{
		DeviceToken: deviceToken,
		Topic:       c.topic,
		Payload:     []byte(`{"aps":{"content-available":1}}`),
		Priority:    apns2.PriorityLow, // background pushes must use priority 5
		PushType:    apns2.PushTypeBackground,
	})
	if err != nil {
		return fmt.Errorf("push: send: %w", err)
	}
	if !res.Sent() {
		return fmt.Errorf("push: rejected: %d %s", res.StatusCode, res.Reason)
	}
	return nil
}

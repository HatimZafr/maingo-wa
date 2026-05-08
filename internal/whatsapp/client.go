package whatsapp

import (
	"context"
	"fmt"
	"strings"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waE2E "go.mau.fi/whatsmeow/binary/proto"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

type MessageHandler func(ctx context.Context, senderPhone string, messageText string) error

type Client struct {
	client    *whatsmeow.Client
	allowlist map[string]bool
	handler   MessageHandler
}

func NewClient(allowlist []string) (*Client, error) {
	dbLog := waLog.Stdout("Database", "WARN", false)
	container, err := sqlstore.New(context.Background(), "sqlite3", "file:whatsmeow.db?_foreign_keys=on", dbLog)
	if err != nil {
		return nil, fmt.Errorf("init sqlstore: %w", err)
	}

	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		return nil, fmt.Errorf("get device: %w", err)
	}

	clientLog := waLog.Stdout("WhatsApp", "INFO", false)
	client := whatsmeow.NewClient(deviceStore, clientLog)
	client.EnableAutoReconnect = true
	client.AutoTrustIdentity = true
	client.AutomaticMessageRerequestFromPhone = true

	al := make(map[string]bool, len(allowlist))
	for _, num := range allowlist {
		al[normalizePhone(num)] = true
	}

	return &Client{
		client:    client,
		allowlist: al,
	}, nil
}

func (c *Client) SetMessageHandler(h MessageHandler) {
	c.handler = h

	c.client.AddEventHandler(func(evt interface{}) {
		switch v := evt.(type) {
		case *events.Message:
			text := v.Message.GetConversation()
			if text == "" {
				return
			}
			phone := extractPhone(v.Info.Sender.User)
			if !c.allowlist[phone] {
				return
			}
			if c.handler != nil {
				_ = c.handler(context.Background(), phone, text)
			}
		case *events.Connected:
			fmt.Println("[WhatsApp] Connected")
		case *events.PairSuccess:
			fmt.Printf("[WhatsApp] Pair success — JID: %s\n", v.ID)
		case *events.LoggedOut:
			fmt.Println("[WhatsApp] Logged out — perlu re-pair")
		case *events.QR:
			for i, code := range v.Codes {
				fmt.Printf("QR Code %d: %s\n", i+1, code)
			}
		}
	})
}

func (c *Client) Connect(ctx context.Context) error {
	return c.client.ConnectContext(ctx)
}

func (c *Client) SendReply(ctx context.Context, recipientPhone string, text string) error {
	jid := types.NewJID(recipientPhone, types.DefaultUserServer)
	msg := &waE2E.Message{
		Conversation: proto.String(text),
	}
	_, err := c.client.SendMessage(ctx, jid, msg)
	if err != nil {
		return fmt.Errorf("send message: %w", err)
	}
	return nil
}

func (c *Client) Disconnect() {
	c.client.Disconnect()
}

func extractPhone(jid string) string {
	parts := strings.Split(jid, "@")
	if len(parts) > 0 {
		return parts[0]
	}
	return jid
}

func normalizePhone(phone string) string {
	return strings.TrimSpace(phone)
}

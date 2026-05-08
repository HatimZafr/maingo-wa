package whatsapp

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	waE2E "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"github.com/mdp/qrterminal/v3"
	"google.golang.org/protobuf/proto"
)

type MessageHandler func(ctx context.Context, senderPhone string, chatJID types.JID, messageID types.MessageID, messageText string) error

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

	clientLog := waLog.Stdout("WhatsApp", "WARN", false)
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
			phone := extractPhone(v.Info.Sender.User)
			conversation := v.Message.GetConversation()
			extended := v.Message.GetExtendedTextMessage()

			var text string
			if conversation != "" {
				text = conversation
			} else if extended != nil {
				text = extended.GetText()
			}

			if text == "" {
				return // non-teks (gambar, stiker, link preview, dll) — abaikan
			}
			if !c.allowlist[phone] {
				return
			}
			if c.handler != nil {
				_ = c.handler(context.Background(), phone, v.Info.Chat, v.Info.ID, text)
			}

		case *events.Connected:
			fmt.Println("[WA] Connected")
		case *events.PairSuccess:
			fmt.Printf("[WA] Pair success — %s\n", v.ID)
		case *events.LoggedOut:
			fmt.Println("[WA] Logged out — restart untuk re-pair")
		case *events.QR:
			if len(v.Codes) > 0 {
				fmt.Println("\n=== Scan QR ini di WhatsApp (Linked Devices) ===")
				qrterminal.GenerateHalfBlock(v.Codes[0], qrterminal.L, os.Stdout)
				fmt.Println()
			}
		case *events.QRScannedWithoutMultidevice:
			fmt.Println("[WA] Multi-Device belum aktif! Buka WhatsApp → Settings → Linked Devices → aktifkan")
		case *events.PairError:
			fmt.Println("[WA] Pairing error — coba scan ulang")
		}
	})
}

func (c *Client) Connect(ctx context.Context) error {
	return c.client.ConnectContext(ctx)
}

func (c *Client) SendTyping(ctx context.Context, chatJID types.JID) error {
	return c.client.SendChatPresence(ctx, chatJID, types.ChatPresenceComposing, types.ChatPresenceMediaText)
}

func (c *Client) MarkRead(ctx context.Context, chatJID, senderJID types.JID, msgID types.MessageID) error {
	return c.client.MarkRead(ctx, []types.MessageID{msgID}, time.Now(), chatJID, senderJID)
}

func (c *Client) SendReply(ctx context.Context, chatJID types.JID, text string) error {
	msg := &waE2E.Message{
		Conversation: proto.String(text),
	}
	_, err := c.client.SendMessage(ctx, chatJID, msg)
	if err != nil {
		return fmt.Errorf("send message: %w", err)
	}
	fmt.Printf("[WA] Reply terkirim ke %s\n", chatJID.User)
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

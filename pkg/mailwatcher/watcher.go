package mailwatcher

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap-idle"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message"
)

// Config configures the mail watcher.
type Config struct {
	Host         string // "host:port", e.g. "imap.gmail.com:993"
	Username     string
	AppPassword  string
	SenderFilter string        // only process emails From this address; empty = no filter
	PollInterval time.Duration // fallback poll interval if the server doesn't support IDLE, and IDLE session refresh interval
}

// Handler is called once per recognized SeaBank transfer-masuk email.
// Returning an error only logs it - the watcher always marks the message
// as read afterwards (see doc comment on processUnseen) since the
// idempotency guarantee lives in the caller's dedup-by-reference-number
// check, not in IMAP \Seen flag bookkeeping.
type Handler func(tx *Transaction) error

// Watcher connects to an IMAP mailbox and invokes Handler for every new
// SeaBank transfer notification email it sees, using IMAP IDLE for
// near-real-time delivery (falling back to polling if the server doesn't
// support IDLE).
type Watcher struct {
	cfg     Config
	handler Handler
}

func New(cfg Config, handler Handler) *Watcher {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 2 * time.Minute
	}
	return &Watcher{cfg: cfg, handler: handler}
}

// Run blocks until ctx is cancelled, maintaining the IMAP connection and
// automatically reconnecting (with backoff) if it drops - a dropped
// network connection, a Gmail-side session timeout, etc. are expected,
// routine occurrences, not fatal errors.
func (w *Watcher) Run(ctx context.Context) error {
	backoff := 5 * time.Second
	const maxBackoff = 2 * time.Minute

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if err := w.runOnce(ctx); err != nil {
			log.Printf("mailwatcher: connection error, reconnecting in %s: %v", backoff, err)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		// Clean exit (ctx cancelled during runOnce) - nothing more to do.
		return nil
	}
}

func (w *Watcher) runOnce(ctx context.Context) error {
	c, err := client.DialTLS(w.cfg.Host, nil)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer c.Logout()

	if err := c.Login(w.cfg.Username, w.cfg.AppPassword); err != nil {
		return fmt.Errorf("login: %w", err)
	}

	if _, err := c.Select("INBOX", false); err != nil {
		return fmt.Errorf("select INBOX: %w", err)
	}

	log.Println("mailwatcher: connected, catching up on unseen messages")
	if err := w.processUnseen(c); err != nil {
		log.Printf("mailwatcher: error processing unseen messages: %v", err)
	}

	// Successfully connected and did the initial catch-up - reset backoff
	// by returning nil only on ctx cancellation; a genuine connection
	// error further down still returns non-nil so Run() backs off.
	idleClient := idle.NewClient(c)
	updates := make(chan client.Update, 10)
	c.Updates = updates

	for {
		stop := make(chan struct{})
		done := make(chan error, 1)
		go func() {
			done <- idleClient.IdleWithFallback(stop, w.cfg.PollInterval)
		}()

		select {
		case <-ctx.Done():
			close(stop)
			<-done
			return nil

		case <-updates:
			// New mailbox activity (could be our target email, could be
			// something else entirely - we don't know until we search).
			close(stop)
			if err := <-done; err != nil {
				return fmt.Errorf("idle: %w", err)
			}
			if err := w.processUnseen(c); err != nil {
				log.Printf("mailwatcher: error processing unseen messages: %v", err)
			}
			// Loop around: start a fresh IDLE session.

		case err := <-done:
			if err != nil {
				return fmt.Errorf("idle: %w", err)
			}
			// IDLE session ended cleanly (e.g. LogoutTimeout refresh) -
			// loop around and start a new one.
		}
	}
}

// processUnseen searches for unread emails from the configured sender,
// parses each one, and invokes the handler for recognized transfer
// notifications.
//
// Every scanned message is marked \Seen afterwards regardless of parse or
// handler outcome. This is deliberate: correctness against
// double-processing (e.g. after a crash-and-restart, or a handler retry)
// is guaranteed by the caller's dedup-by-reference-number check against
// the database, not by IMAP flag state - so there's no need for
// complicated "retry this specific message later" bookkeeping here, and
// no risk of the same unprocessable email being retried forever.
func (w *Watcher) processUnseen(c *client.Client) error {
	criteria := imap.NewSearchCriteria()
	criteria.WithoutFlags = []string{imap.SeenFlag}
	if w.cfg.SenderFilter != "" {
		criteria.Header = map[string][]string{"From": {w.cfg.SenderFilter}}
	}

	ids, err := c.Search(criteria)
	if err != nil {
		return fmt.Errorf("search: %w", err)
	}
	if len(ids) == 0 {
		return nil
	}

	seqset := new(imap.SeqSet)
	seqset.AddNum(ids...)

	section := &imap.BodySectionName{}
	items := []imap.FetchItem{section.FetchItem()}

	messages := make(chan *imap.Message, 10)
	fetchDone := make(chan error, 1)
	go func() {
		fetchDone <- c.Fetch(seqset, items, messages)
	}()

	for msg := range messages {
		body := extractPlainText(msg, section)
		if body == "" {
			log.Println("mailwatcher: could not extract text body from message, skipping")
			continue
		}

		tx, err := ParseSeaBankTransferEmail(body)
		if err != nil {
			// Not every email from this sender is necessarily a transfer
			// notification (could be a statement, a promo, etc.) - skip
			// quietly rather than treating this as an error.
			log.Printf("mailwatcher: skipping unrecognized email: %v", err)
			continue
		}

		if err := w.handler(tx); err != nil {
			log.Printf("mailwatcher: handler error for reference %q: %v", tx.ReferenceNumber, err)
		}
	}

	if err := <-fetchDone; err != nil {
		return fmt.Errorf("fetch: %w", err)
	}

	flagOp := imap.FormatFlagsOp(imap.AddFlags, true)
	if err := c.Store(seqset, flagOp, []interface{}{imap.SeenFlag}, nil); err != nil {
		return fmt.Errorf("mark seen: %w", err)
	}

	return nil
}

// extractPlainText walks a (possibly multipart) email message and returns
// its text/plain body, falling back to a crude HTML-tag-stripped version
// of text/html if no plain-text part exists.
func extractPlainText(msg *imap.Message, section *imap.BodySectionName) string {
	literal := msg.GetBody(section)
	if literal == nil {
		return ""
	}

	entity, err := message.Read(literal)
	if err != nil {
		return ""
	}

	plain, html := findTextParts(entity)
	if plain != "" {
		return plain
	}
	return stripHTMLTags(html)
}

// findTextParts recursively walks a MIME entity and returns the first
// text/plain and text/html bodies found (either may be empty).
func findTextParts(e *message.Entity) (plain, html string) {
	mediaType, _, _ := e.Header.ContentType()

	if strings.HasPrefix(mediaType, "multipart/") {
		mr := e.MultipartReader()
		if mr == nil {
			return "", ""
		}
		for {
			part, err := mr.NextPart()
			if err != nil {
				break
			}
			p, h := findTextParts(part)
			if plain == "" {
				plain = p
			}
			if html == "" {
				html = h
			}
		}
		return plain, html
	}

	data, err := io.ReadAll(e.Body)
	if err != nil {
		return "", ""
	}

	switch mediaType {
	case "text/plain":
		return string(data), ""
	case "text/html":
		return "", string(data)
	default:
		return "", ""
	}
}

var htmlTagStripper = func() func(string) string {
	// Deliberately simple: good enough to turn SeaBank's table-based HTML
	// email into roughly the same line-by-line label/value shape the
	// parser expects from a text/plain body. Not a general-purpose HTML
	// sanitizer.
	return func(s string) string {
		var b strings.Builder
		inTag := false
		for _, r := range s {
			switch {
			case r == '<':
				inTag = true
			case r == '>':
				inTag = false
				b.WriteRune('\n')
			case !inTag:
				b.WriteRune(r)
			}
		}
		return b.String()
	}
}()

func stripHTMLTags(s string) string {
	if s == "" {
		return ""
	}
	return htmlTagStripper(s)
}

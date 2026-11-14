// Package demo provides the same service contract without credentials or network.
package demo

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cliffom/siftpane/internal/core"
)

// Service implements core.Service entirely in memory, with no credential access.
type Service struct {
	mu            sync.Mutex
	conversations []core.Conversation
	messages      map[string][]core.Message
	activity      map[string]core.Message
	sent          map[string]core.Message
	failed        map[string]bool
	polls         int
}

// New seeds distinct conversation histories, threads, and simulated activity.
func New() *Service {
	s := &Service{activity: map[string]core.Message{}, messages: map[string][]core.Message{}, sent: map[string]core.Message{}, failed: map[string]bool{}}
	s.conversations = []core.Conversation{{ID: "Cstudio", Name: "studio", Kind: "channel"}, {ID: "Cshipping", Name: "shipping", Kind: "channel"}, {ID: "Cdesign", Name: "design", Kind: "channel"}, {ID: "Dmaya", Name: "Maya Chen", Kind: "dm"}, {ID: "Dleo", Name: "Leo Park", Kind: "dm"}, {ID: "Gcrew", Name: "Maya, Leo, you", Kind: "group"}, {ID: "Cquiet", Name: "quiet-corner", Kind: "private"}, {ID: "Crandom", Name: "off-topic", Kind: "channel"}}
	seeds := fixtures()
	base := time.Date(2026, 11, 13, 9, 0, 0, 0, time.Local).Unix()
	for ci := range s.conversations {
		c := &s.conversations[ci]
		f := seeds[c.ID]
		c.Topic = f.topic
		c.Members = len(f.authors)
		s.activity[c.ID] = core.Message{Author: f.authors[0], Text: f.activity}
		texts := []string{}
		for _, task := range f.tasks {
			texts = append(texts, "Starting on "+strings.ToLower(task)+" today.", task+" is ready for a second look.", task+" is wrapped up. Thanks for the feedback.")
		}
		texts = append(texts, f.recent...)
		for i, text := range texts {
			ts := fmt.Sprintf("%d.000001", base+int64(i*180))
			author := f.authors[i%len(f.authors)]
			user := map[string]string{"Maya Chen": "U0", "Leo Park": "U1", "You": "U2", "Nora Ellis": "U3"}[author]
			m := core.Message{TS: ts, User: user, Author: author, Text: text}
			if i == 31 {
				m.Replies = 3
				m.Text = f.thread[0]
				replies := []core.Message{m}
				for n := 1; n < 4; n++ {
					replies = append(replies, core.Message{TS: fmt.Sprintf("%d.000001", base+31*180+int64(n*20)), Author: f.authors[n%len(f.authors)], Text: f.thread[n], Thread: ts})
				}
				s.messages[c.ID+"/"+ts] = replies
			}
			s.messages[c.ID] = append(s.messages[c.ID], m)
		}
	}

	return s
}

// Identity returns the fixed offline workspace and demo user.
func (s *Service) Identity(context.Context) (core.Identity, error) {
	return core.Identity{Workspace: "DEMO", WorkspaceName: "Fieldwork · demo", User: "U2", Username: "you"}, nil
}

// Directory returns a page of the demo conversation catalog.
func (s *Service) Directory(_ context.Context, cursor string) (core.DirectoryPage, error) {
	offset, err := pageOffset(cursor, len(s.conversations))
	if err != nil {
		return core.DirectoryPage{}, err
	}
	end := min(offset+4, len(s.conversations))
	p := core.DirectoryPage{Conversations: s.conversations[offset:end]}
	if end < len(s.conversations) {
		p.Next = strconv.Itoa(end)
	}
	return p, nil
}

// History returns a bounded timeline page and occasionally simulates new activity.
func (s *Service) History(_ context.Context, q core.Query) (core.Page, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := q.Conversation
	if q.Thread != "" {
		key += "/" + q.Thread
	}
	ms := s.messages[key]
	if q.Cursor == "" && q.Thread == "" && q.Before == "" {
		s.polls++
		if s.polls%8 == 0 {
			m := s.activity[q.Conversation]
			m.TS = fmt.Sprintf("%d.000001", time.Now().Unix())
			s.messages[key] = append(ms, m)
			ms = s.messages[key]
		}
	}
	if q.Thread != "" && len(ms) == 0 {
		for _, m := range s.messages[q.Conversation] {
			if m.TS == q.Thread {
				ms = []core.Message{m}
				break
			}
		}
	}
	if q.After != "" {
		filtered := []core.Message{}
		for _, m := range ms {
			if m.TS > q.After {
				filtered = append(filtered, m)
			}
		}
		ms = filtered
	}
	if q.Before != "" {
		filtered := []core.Message{}
		for _, m := range ms {
			if m.TS < q.Before {
				filtered = append(filtered, m)
			}
		}
		ms = filtered
	}
	offset, err := pageOffset(q.Cursor, len(ms))
	if err != nil {
		return core.Page{}, err
	}
	end := len(ms) - offset
	if q.Thread != "" {
		end = min(len(ms), offset+15)
		p := core.Page{Messages: append([]core.Message{}, ms[offset:end]...), More: end < len(ms)}
		if p.More {
			p.Next = strconv.Itoa(end)
		}
		return p, nil
	}
	start := max(0, end-15)
	p := core.Page{Messages: append([]core.Message{}, ms[start:end]...), More: start > 0}
	if p.More {
		p.Next = strconv.Itoa(offset + end - start)
	}
	return p, nil
}

// Send records a demo message, failing the first /fail attempt for retry review.
func (s *Service) Send(_ context.Context, o core.Outgoing) (core.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m, ok := s.sent[o.ID]; ok {
		return m, nil
	}
	if strings.Contains(o.Text, "/fail") && !s.failed[o.ID] {
		s.failed[o.ID] = true
		return core.Message{}, sendFailure{}
	}
	m := core.Message{TS: fmt.Sprintf("%d.%06d", time.Now().Unix(), time.Now().Nanosecond()/1000), User: "U2", Author: "You", Text: o.Text, Thread: o.Thread, ClientID: o.ID}
	key := o.Conversation
	if o.Thread != "" {
		key += "/" + o.Thread
		for i := range s.messages[o.Conversation] {
			if s.messages[o.Conversation][i].TS == o.Thread {
				s.messages[o.Conversation][i].Replies++
			}
		}
	}
	s.messages[key] = append(s.messages[key], m)
	s.sent[o.ID] = m
	return m, nil
}

// Conversation returns metadata for one demo conversation.
func (s *Service) Conversation(_ context.Context, id string) (core.Conversation, error) {
	for _, c := range s.conversations {
		if c.ID == id {
			return c, nil
		}
	}
	return core.Conversation{}, errors.New("conversation unavailable")
}

func pageOffset(cursor string, limit int) (int, error) {
	if cursor == "" {
		return 0, nil
	}
	offset, err := strconv.Atoi(cursor)
	if err != nil || offset < 0 || offset > limit {
		return 0, errors.New("invalid demo cursor")
	}
	return offset, nil
}

// sendFailure represents a definite simulated failure, before any message is saved.
type sendFailure struct{}

func (sendFailure) Error() string           { return "demo: simulated send failure; press Esc then r to retry" }
func (sendFailure) DeliveryUncertain() bool { return false }
